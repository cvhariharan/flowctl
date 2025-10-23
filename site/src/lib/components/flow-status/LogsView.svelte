<script lang="ts">
    import { onMount, onDestroy } from "svelte";

    type LogMessage = {
        action_id: string;
        message_type: string;
        node_id: string;
        value: string;
        timestamp: string;
    };

    type Props = {
        logs: string;
        logMessages?: LogMessage[];
        isRunning?: boolean;
        height?: string;
        showCursor?: boolean;
        autoScroll?: boolean;
        showLineNumbers?: boolean;
        theme?: "dark" | "light";
        fontSize?: "xs" | "sm" | "base";
        filterByActionId?: string;
    };

    let {
        logs = $bindable(),
        logMessages = [],
        isRunning = false,
        height = "h-96",
        showCursor = true,
        autoScroll = true,
        showLineNumbers = false,
        theme = "dark",
        fontSize = "sm",
        filterByActionId,
    }: Props = $props();

    let showTimestamp = $state(false);

    let logContainer: HTMLDivElement;
    let previousLogsLength = 0;

    // Color palette for node IDs
    const nodeColors = [
        "text-blue-400",
        "text-green-400",
        "text-yellow-400",
        "text-purple-400",
        "text-pink-400",
        "text-cyan-400",
        "text-orange-400",
        "text-teal-400",
        "text-indigo-400",
        "text-rose-400",
    ];

    let nodeColorMap = $state<Map<string, string>>(new Map());

    const getNodeColor = (nodeId: string): string => {
        if (!nodeColorMap.has(nodeId)) {
            const colorIndex = nodeColorMap.size % nodeColors.length;
            nodeColorMap.set(nodeId, nodeColors[colorIndex]);
        }
        return nodeColorMap.get(nodeId)!;
    };

    const getContainerClasses = () => {
        const baseClasses =
            "rounded-lg p-4 overflow-y-scroll font-mono min-h-128 max-h-128";
        const themeClasses =
            theme === "dark"
                ? "bg-gray-900 text-gray-300"
                : "bg-gray-50 text-gray-900 border border-gray-200";
        const fontClasses = {
            xs: "text-xs",
            sm: "text-sm",
            base: "text-base",
        };

        return `${baseClasses} ${themeClasses} ${fontClasses[fontSize]}`;
    };

    const getCursorClasses = () => {
        const baseClasses = "inline-block";
        const cursorColor =
            theme === "dark" ? "text-primary-400" : "text-primary-600";
        const blinkColor = theme === "dark" ? "text-gray-500" : "text-gray-400";

        return { cursor: cursorColor, blink: blinkColor };
    };

    const scrollToBottom = () => {
        if (logContainer && autoScroll) {
            setTimeout(() => {
                logContainer.scrollTop = logContainer.scrollHeight;
            }, 0);
        }
    };

    const formatLogsWithLineNumbers = (logText: string) => {
        if (!showLineNumbers) return logText;

        return logText
            .split("\n")
            .map(
                (line, index) =>
                    `${(index + 1).toString().padStart(4, " ")} | ${line}`,
            )
            .join("");
    };

    const formatTimestamp = (timestamp: string) => {
        try {
            const date = new Date(timestamp);
            return date.toLocaleTimeString("en-US", { hour12: false });
        } catch {
            return timestamp;
        }
    };

    const formatLogMessage = (msg: LogMessage) => {
        const lines = msg.value
            .split("\n")
            .filter((line) => line.trim() !== "");
        return lines.map((line) => ({
            timestamp:
                showTimestamp && msg.timestamp
                    ? formatTimestamp(msg.timestamp)
                    : null,
            nodeId: msg.node_id,
            nodeColor: getNodeColor(msg.node_id),
            value: line,
        }));
    };

    const filteredLogMessages = $derived(() => {
        if (!logMessages || logMessages.length === 0) return [];
        if (!filterByActionId) return logMessages;
        return logMessages.filter((msg) => msg.action_id === filterByActionId);
    });

    const hasStructuredLogs = $derived(() => {
        return logMessages && logMessages.length > 0;
    });

    const formattedStructuredLogs = $derived(() => {
        if (!hasStructuredLogs()) return [];
        const messagesToUse = filterByActionId
            ? filteredLogMessages()
            : logMessages;
        return messagesToUse.flatMap((msg) => formatLogMessage(msg));
    });

    const processedRawLogs = $derived(() => {
        return formatLogsWithLineNumbers(logs);
    });

    const cursorClasses = $derived(getCursorClasses());

    // Auto-scroll when logs change
    $effect(() => {
        const currentLength = hasStructuredLogs()
            ? logMessages.length
            : logs.length;
        if (currentLength > previousLogsLength) {
            scrollToBottom();
            previousLogsLength = currentLength;
        }
    });

    // Scroll to bottom on mount
    onMount(() => {
        scrollToBottom();
    });
</script>

<div class="flex flex-col space-y-3">
    <!-- Controls -->
    {#if logMessages && logMessages.length > 0}
        <div class="flex gap-4 text-sm flex-shrink-0">
            <label class="flex items-center gap-2 cursor-pointer">
                <input
                    type="checkbox"
                    bind:checked={showTimestamp}
                    class="rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                />
                <span class="text-gray-900">Show Timestamp</span>
            </label>
        </div>
    {/if}

    <!-- Log Terminal -->
    <div class={getContainerClasses()} bind:this={logContainer}>
        {#if filterByActionId && filteredLogMessages().length === 0}
            <div
                class="flex items-center justify-center h-full text-gray-500 text-sm"
            >
                No logs available for this action
            </div>
        {:else}
            <div class="whitespace-pre-wrap break-words">
                {#if hasStructuredLogs()}
                    {#each formattedStructuredLogs() as logMsg}
                        {#if logMsg.timestamp}<span class="text-gray-500"
                                >[{logMsg.timestamp}]</span
                            >
                        {/if}{#if logMsg.nodeId}<span
                                class="font-semibold {logMsg.nodeColor}"
                                >[{logMsg.nodeId}]</span
                            >
                        {/if}{logMsg.value}<br />
                    {/each}
                {:else}
                    {processedRawLogs()}
                {/if}
                {#if isRunning && showCursor}
                    <div class="inline-block">
                        <span class={cursorClasses.cursor}>█</span>
                        <span class="animate-pulse {cursorClasses.blink}"
                            >_</span
                        >
                    </div>
                {/if}
            </div>
        {/if}
    </div>
</div>
