export type LogMessageType =
    | "log"
    | "result"
    | "error"
    | "cancelled"
    | "approval";

export type LogMessage = {
    message_type: LogMessageType;
    action_id: string;
    node_id: string;
    value: string;
    timestamp: string;
    results?: Record<string, string>;
};

export type ServerFrame =
    | { type: "open"; protocol: number; exec_id: string; replay: boolean }
    | { type: "batch"; messages: LogMessage[] }
    | { type: "ping" }
    | { type: "end"; reason: "complete" | "timeout" };

type LogStreamOptions = {
    namespace: string;
    logId: string;
    onReset: () => void;
    onMessages: (messages: LogMessage[]) => void;
    onEnd: (reason: "complete" | "timeout") => boolean | void | Promise<boolean | void>;
    onFatal: (error: Error) => void;
};

export type LogStream = { close(): void };

const PROTOCOL_VERSION = 1;
const WATCHDOG_MS = 45_000;
const INITIAL_BACKOFF_MS = 500;
const MAX_BACKOFF_MS = 8_000;

export function createLogStream(options: LogStreamOptions): LogStream {
    let socket: WebSocket | null = null;
    let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
    let watchdogTimer: ReturnType<typeof setTimeout> | null = null;
    let backoffMs = INITIAL_BACKOFF_MS;
    let closedByCaller = false;
    let fatal = false;
    let everOpened = false;
    let connectionID = 0;

    const clearWatchdog = () => {
        if (watchdogTimer !== null) {
            clearTimeout(watchdogTimer);
            watchdogTimer = null;
        }
    };

    const scheduleReconnect = () => {
        if (closedByCaller || fatal || reconnectTimer !== null) return;
        const jitter = 0.8 + Math.random() * 0.4;
        const delay = Math.round(backoffMs * jitter);
        backoffMs = Math.min(backoffMs * 2, MAX_BACKOFF_MS);
        reconnectTimer = setTimeout(() => {
            reconnectTimer = null;
            connect();
        }, delay);
    };

    const resetWatchdog = (ws: WebSocket, id: number) => {
        clearWatchdog();
        watchdogTimer = setTimeout(() => {
            if (socket !== ws || connectionID !== id || closedByCaller || fatal) return;
            socket = null;
            connectionID++;
            ws.close(1000, "liveness timeout");
            scheduleReconnect();
        }, WATCHDOG_MS);
    };

    const stopFatally = (error: Error, ws?: WebSocket) => {
        fatal = true;
        clearWatchdog();
        if (reconnectTimer !== null) {
            clearTimeout(reconnectTimer);
            reconnectTimer = null;
        }
        if (ws && ws.readyState < WebSocket.CLOSING) ws.close(1000, "fatal error");
        options.onFatal(error);
    };

    const connect = () => {
        if (closedByCaller || fatal) return;

        const scheme = location.protocol === "https:" ? "wss:" : "ws:";
        const namespace = encodeURIComponent(options.namespace);
        const logId = encodeURIComponent(options.logId);
        const ws = new WebSocket(
            `${scheme}//${location.host}/api/v1/${namespace}/logs/${logId}/stream`,
        );
        const id = ++connectionID;
        socket = ws;
        let receivedOpen = false;
        let receivedEnd = false;
        let failedBeforeOpen = false;

        ws.onopen = () => {
            if (socket !== ws || id !== connectionID) return;
            everOpened = true;
            resetWatchdog(ws, id);
        };

        ws.onmessage = async (event) => {
            if (socket !== ws || id !== connectionID) return;
            resetWatchdog(ws, id);

            let frame: ServerFrame;
            try {
                frame = JSON.parse(String(event.data)) as ServerFrame;
            } catch {
                return;
            }
            if (!frame || typeof frame !== "object" || !("type" in frame)) return;

            switch (frame.type) {
                case "open":
                    if (frame.protocol !== PROTOCOL_VERSION) {
                        stopFatally(
                            new Error("The log stream protocol changed. Reload this page to continue."),
                            ws,
                        );
                        return;
                    }
                    receivedOpen = true;
                    backoffMs = INITIAL_BACKOFF_MS;
                    options.onReset();
                    break;
                case "batch":
                    if (receivedOpen && Array.isArray(frame.messages) && frame.messages.length > 0) {
                        options.onMessages(frame.messages);
                    }
                    break;
                case "ping":
                    break;
                case "end": {
                    receivedEnd = true;
                    clearWatchdog();
                    socket = null;
                    const reconnect = await options.onEnd(frame.reason);
                    if (frame.reason === "timeout" && reconnect === true) scheduleReconnect();
                    break;
                }
                default:
                    break;
            }
        };

        ws.onerror = () => {
            if (!receivedOpen && !everOpened) failedBeforeOpen = true;
        };

        ws.onclose = () => {
            if (id !== connectionID) return;
            clearWatchdog();
            if (socket === ws) socket = null;
            if (closedByCaller || fatal || receivedEnd) return;
            if (failedBeforeOpen) {
                stopFatally(new Error("The server rejected the log stream connection."));
                return;
            }
            scheduleReconnect();
        };
    };

    connect();

    return {
        close() {
            closedByCaller = true;
            connectionID++;
            clearWatchdog();
            if (reconnectTimer !== null) {
                clearTimeout(reconnectTimer);
                reconnectTimer = null;
            }
            const ws = socket;
            socket = null;
            if (ws && ws.readyState < WebSocket.CLOSING) ws.close(1000, "client closed");
        },
    };
}
