<script lang="ts">
  import { onMount, onDestroy } from 'svelte';

  type LogMessage = {
    action_id: string;
    message_type: string;
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
    theme?: 'dark' | 'light';
    fontSize?: 'xs' | 'sm' | 'base';
    filterByActionId?: string;
  };

  let {
    logs = $bindable(),
    logMessages = [],
    isRunning = false,
    height = 'h-96',
    showCursor = true,
    autoScroll = true,
    showLineNumbers = false,
    theme = 'dark',
    fontSize = 'sm',
    filterByActionId
  }: Props = $props();

  let showTimestamp = $state(false);

  let logContainer: HTMLDivElement;
  let previousLogsLength = 0;

  const getContainerClasses = () => {
    const baseClasses = 'rounded-lg p-4 overflow-y-auto font-mono';
    const themeClasses = theme === 'dark' 
      ? 'bg-gray-900 text-gray-300' 
      : 'bg-gray-50 text-gray-900 border border-gray-200';
    const fontClasses = {
      xs: 'text-xs',
      sm: 'text-sm',
      base: 'text-base'
    };

    return `${baseClasses} ${themeClasses} ${fontClasses[fontSize]} ${height}`;
  };

  const getCursorClasses = () => {
    const baseClasses = 'inline-block';
    const cursorColor = theme === 'dark' ? 'text-primary-400' : 'text-primary-600';
    const blinkColor = theme === 'dark' ? 'text-gray-500' : 'text-gray-400';
    
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
      .split('\n')
      .map((line, index) => `${(index + 1).toString().padStart(4, ' ')} | ${line}`)
      .join('');
  };

  const formatTimestamp = (timestamp: string) => {
    try {
      const date = new Date(timestamp);
      return date.toLocaleTimeString('en-US', { hour12: false });
    } catch {
      return timestamp;
    }
  };

  const formatLogMessage = (msg: LogMessage) => {
    let parts: string[] = [];

    if (showTimestamp && msg.timestamp) {
      parts.push(`[${formatTimestamp(msg.timestamp)}]`);
    }

    parts.push(msg.value);

    return parts.join(' ');
  };

  const filteredLogMessages = $derived(() => {
    if (!logMessages || logMessages.length === 0) return [];
    if (!filterByActionId) return logMessages;
    return logMessages.filter(msg => msg.action_id === filterByActionId);
  });

  const processedLogs = $derived(() => {
    // If we have structured log messages, use them
    if (logMessages && logMessages.length > 0) {
      const messagesToUse = filterByActionId ? filteredLogMessages() : logMessages;
      const formatted = messagesToUse.map(msg => formatLogMessage(msg)).join('');
      return formatLogsWithLineNumbers(formatted);
    }
    // Otherwise fall back to raw logs
    return formatLogsWithLineNumbers(logs);
  });

  const cursorClasses = $derived(getCursorClasses());

  // Auto-scroll when logs change
  $effect(() => {
    const currentLength = logMessages && logMessages.length > 0 ? logMessages.length : logs.length;
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

<div class="flex flex-col space-y-3 h-full">
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
  <div class="flex-1 min-h-0">
    <div class={getContainerClasses()} bind:this={logContainer}>
    {#if filterByActionId && filteredLogMessages().length === 0}
      <div class="flex items-center justify-center h-full text-gray-500 text-sm">
        No logs available for this action
      </div>
    {:else}
      <div class="whitespace-pre-wrap break-words">
        {processedLogs()}
        {#if isRunning && showCursor}
          <div class="inline-block">
            <span class={cursorClasses.cursor}>█</span>
            <span class="animate-pulse {cursorClasses.blink}">_</span>
          </div>
        {/if}
      </div>
    {/if}
    </div>
  </div>
</div>