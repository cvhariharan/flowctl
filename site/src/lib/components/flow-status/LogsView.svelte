<script lang="ts">
    import { onMount, tick } from "svelte";
    import { formatTime } from "$lib/utils";
    import Convert from 'ansi-to-html';
    import { IconDownload } from '@tabler/icons-svelte';

    const convert = new Convert({ escapeXML: true });
    const containsAnsi = (text: string) => text.includes('\x1b[');
    const ansiToHtml = (text: string) => convert.toHtml(text);

    type LogMessage = {
        action_id: string;
        message_type: string;
        node_id: string;
        value: string;
        timestamp: string;
    };

    type FormattedLog = {
        timestamp: string | null;
        nodeId: string;
        nodeColor: string;
        value: string;
        html: string;
        hasAnsi: boolean;
    };

    type Props = {
        logMessages?: LogMessage[];
        isRunning?: boolean;
        height?: string;
        autoScroll?: boolean;
        theme?: "dark" | "light";
        fontSize?: "xs" | "sm" | "base";
        filterByActionId?: string;
        logId?: string;
        namespace?: string;
        showControls?: boolean;
        showTimestamp?: boolean;
    };

    let {
        logMessages = [],
        isRunning = false,
        height = "h-96",
        autoScroll = $bindable(true),
        theme = "dark",
        fontSize = "sm",
        filterByActionId,
        logId,
        namespace,
        showControls = true,
        showTimestamp = $bindable(false),
    }: Props = $props();

    const canDownload = $derived(!isRunning && !!logId && !!namespace);

    const downloadLogs = () => {
        if (!logId || !namespace) return;
        const a = document.createElement("a");
        a.href = `/api/v1/${encodeURIComponent(namespace)}/logs/${logId}/download`;
        a.download = `${logId}.log`;
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
    };

    let scrollContainer: HTMLDivElement | undefined;

    const ITEM_HEIGHT = 20;
    const BUFFER_SIZE = 30;
    let scrollTop = $state(0);
    let viewportHeight = $state(500);

    let nodeColorMap = new Map<string, string>();
    const nodeColors = [
        "node-blue",
        "node-green",
        "node-yellow",
        "node-purple",
        "node-pink",
        "node-cyan",
        "node-orange",
        "node-teal",
        "node-indigo",
        "node-rose",
    ];

    const getNodeColor = (nodeId: string): string => {
        if (!nodeColorMap.has(nodeId)) {
            const colorIndex = nodeColorMap.size % nodeColors.length;
            nodeColorMap.set(nodeId, nodeColors[colorIndex]);
        }
        return nodeColorMap.get(nodeId)!;
    };

    const filteredMessages = $derived(
        filterByActionId
            ? logMessages.filter((msg) => msg.action_id === filterByActionId)
            : logMessages
    );

    const processedLogs = $derived.by(() => {
        const messages = filteredMessages;
        if (!messages || messages.length === 0) return [];

        nodeColorMap = new Map<string, string>();

        const result: FormattedLog[] = [];
        for (const msg of messages) {
            const lines = msg.value.split("\n").filter((line) => line.trim() !== "");
            for (const line of lines) {
                const hasAnsi = containsAnsi(line);
                result.push({
                    timestamp: msg.timestamp ? formatTime(msg.timestamp) : null,
                    nodeId: msg.node_id,
                    nodeColor: getNodeColor(msg.node_id),
                    value: line,
                    hasAnsi,
                    html: hasAnsi ? ansiToHtml(line) : '',
                });
            }
        }
        return result;
    });

    const totalHeight = $derived(processedLogs.length * ITEM_HEIGHT);
    const startIndex = $derived(Math.max(0, Math.floor(scrollTop / ITEM_HEIGHT) - BUFFER_SIZE));
    const endIndex = $derived(Math.min(processedLogs.length, Math.ceil((scrollTop + viewportHeight) / ITEM_HEIGHT) + BUFFER_SIZE));
    const visibleLogs = $derived(processedLogs.slice(startIndex, endIndex));
    const offsetY = $derived(startIndex * ITEM_HEIGHT);

    const handleScroll = (e: Event) => {
        const target = e.target as HTMLDivElement;
        scrollTop = target.scrollTop;
    };

    let lastLogCount = 0;
    $effect(() => {
        const currentCount = processedLogs.length;
        if (autoScroll && currentCount > lastLogCount && scrollContainer) {
            lastLogCount = currentCount;
            tick().then(() => {
                if (scrollContainer) {
                    scrollContainer.scrollTop = scrollContainer.scrollHeight;
                }
            });
        }
    });

    onMount(() => {
        if (scrollContainer) {
            viewportHeight = scrollContainer.clientHeight;

            const observer = new ResizeObserver((entries) => {
                for (const entry of entries) {
                    viewportHeight = entry.contentRect.height;
                }
            });
            observer.observe(scrollContainer);

            return () => observer.disconnect();
        }
    });
</script>

<div class="vstack gap-2" style="height: 100%;">
    {#if showControls && ((logMessages && logMessages.length > 0) || canDownload)}
        <div class="hstack toolbar">
            <div class="hstack gap-4">
                {#if logMessages && logMessages.length > 0}
                    <label class="hstack gap-2 checkbox-label">
                        <input
                            type="checkbox"
                            bind:checked={showTimestamp}
                        />
                        <span>Show Timestamp</span>
                    </label>
                {/if}
            </div>
            {#if canDownload}
                <button
                    data-variant="secondary"
                    class="download-btn"
                    onclick={downloadLogs}
                >
                    <IconDownload size={12} />
                    Download
                </button>
            {/if}
        </div>
    {/if}

    <div
        class="log-container"
        class:theme-dark={theme === 'dark'}
        class:theme-light={theme === 'light'}
        class:text-xs={fontSize === 'xs'}
        class:text-sm={fontSize === 'sm'}
        class:text-base={fontSize === 'base'}
        bind:this={scrollContainer}
        onscroll={handleScroll}
    >
        {#if filterByActionId && processedLogs.length === 0 && !isRunning}
            <div class="centered-msg text-lighter">
                No logs available for this action
            </div>
        {:else if processedLogs.length > 0}
            <div style="height: {totalHeight}px; width: 100%; position: relative;">
                <div style="position: absolute; top: 0; left: 0; width: 100%; transform: translateY({offsetY}px);">
                    {#each visibleLogs as logMsg, i (startIndex + i)}
                        <div class="log-line" style="height: {ITEM_HEIGHT}px; line-height: {ITEM_HEIGHT}px;">
                            {#if showTimestamp && logMsg.timestamp}<span class="timestamp">[{logMsg.timestamp}]</span>{/if}{#if logMsg.nodeId}<span class="node-id {logMsg.nodeColor}">[{logMsg.nodeId}]</span>{/if}{#if logMsg.hasAnsi}{@html logMsg.html}{:else}{logMsg.value}{/if}
                        </div>
                    {/each}
                </div>
            </div>
            {#if isRunning}
                <div class="cursor-block">
                    <span class="cursor-char">&#x2588;</span>
                    <span class="cursor-blink">_</span>
                </div>
            {/if}
        {:else}
            <div class="centered-msg text-lighter">
                {#if isRunning}
                    Waiting for logs...
                {:else}
                    No logs available
                {/if}
            </div>
        {/if}
    </div>
</div>

<style>
    .toolbar {
        justify-content: space-between;
        gap: 1rem;
        font-size: 0.875rem;
        flex-shrink: 0;
    }
    .checkbox-label {
        cursor: pointer;
    }
    .download-btn {
        display: flex;
        align-items: center;
        gap: var(--space-2);
        font-size: var(--text-7);
    }
    .log-container {
        border-radius: var(--radius-medium);
        padding: var(--space-3);
        font-family: monospace;
        height: 100%;
        overflow-y: auto;
        overflow-x: auto;
    }
    .theme-dark {
        background: var(--background);
        color: var(--foreground);
    }
    .theme-light {
        background: var(--faint);
        color: var(--foreground);
        border: 1px solid var(--border);
    }
    .text-xs { font-size: 0.75rem; }
    .text-sm { font-size: 0.875rem; }
    .text-base { font-size: 1rem; }
    .log-line {
        white-space: nowrap;
    }
    .timestamp {
        color: var(--muted-foreground);
    }
    .node-id {
        font-weight: 600;
    }
    .node-blue,
    .node-indigo { color: var(--primary); }
    .node-green,
    .node-teal { color: var(--success); }
    .node-yellow,
    .node-orange { color: var(--warning); }
    .node-purple,
    .node-pink { color: var(--danger); }
    .node-cyan { color: var(--info); }
    .node-rose { color: var(--neutral); }
    .centered-msg {
        display: flex;
        align-items: center;
        justify-content: center;
        height: 100%;
        font-size: 0.875rem;
    }
    .cursor-block {
        display: inline-block;
    }
    .cursor-char {
        color: var(--primary);
    }
    .cursor-blink {
        animation: pulse 2s infinite;
        color: var(--muted-foreground);
    }
    @keyframes pulse {
        0%, 100% { opacity: 1; }
        50% { opacity: 0.5; }
    }
</style>
