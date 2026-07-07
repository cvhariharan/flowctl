<script lang="ts">
    import { onMount, onDestroy } from "svelte";
    import { goto } from "$app/navigation";
    import Header from "$lib/components/shared/Header.svelte";
    import StatusBadge from "$lib/components/shared/StatusBadge.svelte";
    import ActionsList from "$lib/components/flow-status/ActionsList.svelte";
    import LogsView from "$lib/components/flow-status/LogsView.svelte";
    import ExecutionOutputTable from "$lib/components/flow-status/ExecutionOutputTable.svelte";
    import type { FlowMetaResp, ExecutionSummary, ArtifactMetadata } from "$lib/types";
    import { apiClient, ApiError } from "$lib/apiClient";
    import {
        handleInlineError,
        showInfo,
        showSuccess,
        showWarning,
    } from "$lib/utils/errorHandling";
    import { formatDateTime, getStartTime } from "$lib/utils";
    import { IconPlayerStop, IconRefresh, IconRepeat } from "@tabler/icons-svelte";

    let {
        data,
    }: {
        data: {
            namespace: string;
            flowId: string;
            logId: string;
            flowMeta?: FlowMetaResp;
            executionSummary?: ExecutionSummary;
            error?: string;
        };
    } = $props();

    // Flow execution state
    let status = $state<
        "running" | "completed" | "awaiting_approval" | "errored" | "cancelled"
    >("running");
    let currentActionIndex = $state(-1);
    let completedActions = $state<number[]>([]);
    let failedActionIndex = $state(-1);
    let logOutput = $state("");
    let logMessages = $state<
        Array<{
            action_id: string;
            message_type: string;
            node_id: string;
            value: string;
            timestamp: string;
        }>
    >([]);
    let results = $state<Record<string, any>>({});
    let showApproval = $state(false);
    let approvalID = $state<string | null>(null);
    let selectedActionId = $state<string>("");
    let startTime = $state("");
    let flowName = $state("");

    // SSE connection
    let eventSource: EventSource | null = null;
    let hasReceivedMessages = $state(false);
    let manuallyClosed = $state(false);

    // Retry state
    let isRetrying = $state(false);
    let retryPollingInterval: ReturnType<typeof setInterval> | null = null;

    // Message batching for performance
    let messageBuffer: Array<{
        action_id: string;
        message_type: string;
        node_id: string;
        value: string;
        timestamp: string;
    }> = [];
    let logOutputBuffer: string[] = [];
    let rafId: number | null = null;

    const flushMessageBuffer = () => {
        rafId = null;
        if (messageBuffer.length === 0 && logOutputBuffer.length === 0) return;

        if (messageBuffer.length > 0) {
            const newMessages = [...messageBuffer];
            messageBuffer = [];
            logMessages = logMessages.concat(newMessages);
        }

        if (logOutputBuffer.length > 0) {
            const newOutput = logOutputBuffer.join("");
            logOutputBuffer = [];
            logOutput = logOutput + newOutput;
        }
    };

    const scheduleFlush = () => {
        if (rafId === null) {
            rafId = requestAnimationFrame(flushMessageBuffer);
        }
    };

    const addLogMessage = (msg: {
        action_id: string;
        message_type: string;
        node_id: string;
        value: string;
        timestamp: string;
    }) => {
        messageBuffer.push(msg);
        logOutputBuffer.push((msg.value || "") + "\n");
        scheduleFlush();
    };

    const resetLogState = () => {
        if (rafId !== null) {
            cancelAnimationFrame(rafId);
            rafId = null;
        }
        messageBuffer = [];
        logOutputBuffer = [];
        logMessages = [];
        logOutput = "";
    };

    let artifacts = $state<ArtifactMetadata[]>([]);

    const loadArtifacts = async () => {
        if (!logId) return;
        try {
            artifacts = await apiClient.executions.getArtifacts(namespace, logId);
        } catch (error) {
            console.error("Failed to load artifacts:", error);
        }
    };

    // Polling for execution status updates
    let statusPollingInterval: NodeJS.Timeout | null = null;

    // Derived values
    let namespace = $derived(data.namespace);
    let flowId = $derived(data.flowId);
    let logId = $derived(data.logId);
    let actions = $derived(data.flowMeta?.actions || []);
    let flowHasAutoRetry = $derived((data.flowMeta?.meta?.max_retries ?? 0) > 0);

    // Transform actions into list items with status
    let actionsList = $derived(
        actions.map((action, index) => ({
            id: action.id,
            name: action.name || `Action ${index + 1}`,
            status: getActionStatus(index),
        })),
    );

    const updateExecutionStatus = async () => {
        if (!logId) return;
        try {
            const executionSummary = await apiClient.executions.getById(
                namespace,
                logId,
            );
            updateStatusFromSummary(executionSummary);
        } catch (error) {
            // Silently handle errors during polling to avoid spam
            console.error("Failed to fetch execution summary:", error);
        }
    };

    const startStatusPolling = () => {
        // Always stop any existing polling first
        stopStatusPolling();
        // Poll every 2 seconds when flow is active
        if (status === "running" || status === "awaiting_approval") {
            statusPollingInterval = setInterval(updateExecutionStatus, 2000);
        }
    };

    const stopStatusPolling = () => {
        if (statusPollingInterval) {
            clearInterval(statusPollingInterval);
            statusPollingInterval = null;
        }
    };

    const updateStatusFromSummary = (executionSummary: ExecutionSummary) => {
        const execStatus = executionSummary.status;
        let newStatus: typeof status;

        if (execStatus === "pending" || execStatus === "running") {
            newStatus = "running";
        } else if (execStatus === "pending_approval") {
            newStatus = "awaiting_approval";
            approvalID = executionSummary.current_action_id; // Use current_action_id for approval context
            showApproval = true;
        } else if (execStatus === "cancelled") {
            newStatus = "cancelled";
        } else if (execStatus === "completed") {
            newStatus = "completed";
        } else if (execStatus === "errored") {
            newStatus = "errored";
        } else {
            newStatus = "running";
        }

        // Update status and reconstruct progress
        status = newStatus;
        if (executionSummary.current_action_id) {
            reconstructProgress(
                executionSummary.current_action_id,
                executionSummary.status,
            );
        }

        // Start/stop polling based on status
        if (
            newStatus === "completed" ||
            newStatus === "errored" ||
            newStatus === "cancelled"
        ) {
            stopStatusPolling();
            loadArtifacts();
        } else {
            startStatusPolling();
        }

        // Reconnect SSE if status changed to running but not connected (scheduler retry)
        if (newStatus === "running" && !eventSource) {
            resetLogState();
            connectSSE();
        }
    };

    const connectSSE = () => {
        const sseUrl = `/api/v1/${encodeURIComponent(namespace)}/logs/${logId}`;
        eventSource = new EventSource(sseUrl);

        eventSource.onmessage = (event) => {
            hasReceivedMessages = true;
            let msg = {};
            try {
                msg = JSON.parse(event.data);
            } catch (e) {
                handleInlineError(e, "SSE Message Parse Error");
            }
            processMessage(msg);
        };

        eventSource.addEventListener("end", () => {
            if (eventSource) {
                eventSource.close();
                eventSource = null;
            }
            updateExecutionStatus();
        });

        eventSource.onerror = () => {
            if (manuallyClosed) {
                return;
            }

            if (eventSource?.readyState === EventSource.CLOSED) {
                updateExecutionStatus();
            }
        };
    };

    const reconstructProgress = (
        currentActionId: string,
        executionStatus: string,
    ) => {
        let actionIndex = actions.findIndex(
            (action) => action.id === currentActionId,
        );
        if (actionIndex === -1) return;

        for (let i = 0; i < actionIndex; i++) {
            if (!completedActions.includes(i)) {
                completedActions.push(i);
            }
        }

        if (executionStatus === "completed") {
            for (let i = 0; i < actions.length; i++) {
                if (!completedActions.includes(i)) {
                    completedActions.push(i);
                }
            }
            currentActionIndex = -1;
        } else if (executionStatus === "errored") {
            failedActionIndex = actionIndex;
            currentActionIndex = -1;
        } else if (executionStatus === "cancelled") {
            failedActionIndex = actionIndex;
            currentActionIndex = -1;
            status = "cancelled";
        } else if (
            executionStatus === "running" ||
            executionStatus === "pending"
        ) {
            currentActionIndex = actionIndex;
        } else if (executionStatus === "pending_approval") {
            currentActionIndex = actionIndex;
            status = "awaiting_approval";
        }
    };

    const processMessage = (msg: any) => {
        if (msg.action_id) {
            const actionIndex = actions.findIndex(
                (a) => a.id === msg.action_id,
            );
            if (actionIndex !== -1) {
                // Only process action transitions when moving forward (ignores replayed old messages)
                if (actionIndex > currentActionIndex) {
                    // Mark previous action as completed when moving to next action
                    if (
                        currentActionIndex !== -1 &&
                        !completedActions.includes(currentActionIndex)
                    ) {
                        completedActions.push(currentActionIndex);
                    }
                    currentActionIndex = actionIndex;
                } else if (currentActionIndex === -1) {
                    // Initialize currentActionIndex if not set
                    currentActionIndex = actionIndex;
                }
            }
        }

        switch (msg.message_type) {
            case "log":
                addLogMessage({
                    action_id: msg.action_id || "",
                    message_type: msg.message_type,
                    node_id: msg.node_id || "",
                    value: msg.value || "",
                    timestamp: msg.timestamp || "",
                });
                break;
            case "result":
                results = { ...results, ...(msg.results || {}) };
                break;
            case "error":
                flushMessageBuffer();
                if (msg.value && msg.value.includes("cancelled")) {
                    status = "cancelled";
                } else {
                    handleInlineError(
                        new ApiError(500, "Flow execution failed", {
                            error: msg.value || "An error occurred.",
                            code: "OPERATION_FAILED",
                        }),
                        "Flow Execution Error",
                    );
                    status = "errored";
                }
                if (currentActionIndex !== -1) {
                    failedActionIndex = currentActionIndex;
                }
                stopStatusPolling();
                break;
            case "approval":
                flushMessageBuffer();
                approvalID = msg.value;
                showApproval = true;
                status = "awaiting_approval";
                stopStatusPolling();
                break;
            case "completed":
                flushMessageBuffer();
                status = "completed";
                if (eventSource) {
                    eventSource.close();
                    eventSource = null;
                }
                stopStatusPolling();
                updateExecutionStatus();
                break;
            case "cancelled":
                flushMessageBuffer();
                status = "cancelled";
                addLogMessage({
                    action_id: msg.action_id || "",
                    message_type: msg.message_type,
                    node_id: msg.node_id || "",
                    value: msg.value || "Flow execution was cancelled",
                    timestamp: msg.timestamp || "",
                });
                flushMessageBuffer();
                if (eventSource) {
                    eventSource.close();
                    eventSource = null;
                }
                stopStatusPolling();
                break;
            default:
                addLogMessage({
                    action_id: msg.action_id || "",
                    message_type: msg.message_type,
                    node_id: msg.node_id || "",
                    value: msg.value || "",
                    timestamp: msg.timestamp || "",
                });
        }
    };

    const goBack = () => {
        goto(`/view/${encodeURIComponent(namespace)}/flows`);
    };

    const getActionStatus = (
        index: number,
    ):
        | "pending"
        | "running"
        | "completed"
        | "failed"
        | "awaiting_approval"
        | "cancelled" => {
        // Handle completed actions - they should always stay green
        if (completedActions.includes(index)) return "completed";

        // Handle failed action
        if (index === failedActionIndex) return "failed";

        // Handle current action based on flow status
        if (index === currentActionIndex) {
            if (status === "running") return "running";
            if (status === "awaiting_approval") return "awaiting_approval";
            if (status === "cancelled") return "cancelled";
            if (status === "errored") return "failed";
        }

        // Special case: if flow is awaiting approval and no current action is set,
        // find the first non-completed action to mark as awaiting approval
        if (status === "awaiting_approval" && currentActionIndex === -1) {
            const firstIncompleteIndex = actions.findIndex(
                (_, i) => !completedActions.includes(i),
            );
            if (index === firstIncompleteIndex) return "awaiting_approval";
        }

        // Handle remaining actions based on flow status
        const lastProcessedIndex = Math.max(
            currentActionIndex >= 0 ? currentActionIndex : -1,
            failedActionIndex >= 0 ? failedActionIndex : -1,
            completedActions.length > 0 ? Math.max(...completedActions) : -1,
        );

        // If flow has failed, errored, or cancelled, actions after the failure/cancellation point should be cancelled
        if (
            (status === "errored" || status === "cancelled") &&
            index > lastProcessedIndex
        ) {
            return "cancelled";
        }

        // Default to pending for actions that haven't started yet
        return "pending";
    };

    const dismissApproval = () => {
        showApproval = false;
    };

    const handleActionSelect = (actionId: string) => {
        selectedActionId = actionId;
    };

    const stopFlow = async () => {
        try {
            await apiClient.executions.cancel(namespace, logId);

            status = "cancelled";

            manuallyClosed = true;
            if (eventSource) {
                eventSource.close();
            }

            showWarning(
                "Flow Cancellation",
                "Flow cancellation signal has been sent",
            );
        } catch (error) {
            handleInlineError(error, "Unable to Cancel Flow");
        }
    };

    const handleRerun = () => {
        goto(`/view/${encodeURIComponent(namespace)}/flows/${flowId}?rerun_from=${logId}`);
    };

    const handleRetry = async () => {
        if (isRetrying) return;

        try {
            isRetrying = true;

            // Close current SSE connection to stop old logs
            if (eventSource) {
                eventSource.close();
                eventSource = null;
            }

            // Stop current status polling
            stopStatusPolling();

            // Capture current retry counts before calling retry
            const preRetryState = await apiClient.executions.getById(namespace, logId);
            const preRetryCount = preRetryState.action_retries?.[preRetryState.current_action_id] || 0;

            await apiClient.executions.retry(namespace, logId);
            showInfo("Execution Retry", "Retrying execution...");
            startRetryPolling(preRetryState.current_action_id, preRetryCount);

        } catch (error) {
            isRetrying = false;
            handleInlineError(error, "Unable to Retry Execution");
        }
    };

    const startRetryPolling = (actionId: string, baselineRetryCount: number) => {
        let pollAttempts = 0;
        const maxPollAttempts = 15; // 30 seconds

        // Poll every 2 seconds
        retryPollingInterval = setInterval(async () => {
            pollAttempts++;
            try {
                const currentState = await apiClient.executions.getById(namespace, logId);

                if (pollAttempts >= maxPollAttempts) {
                    stopRetryPolling();
                    isRetrying = false;
                    handleInlineError(
                        new Error("Retry timeout"),
                        "Retry timed out - execution may still be queued"
                    );
                    return;
                }

                // Check if retry count for the action has increased
                const currentRetryCount = currentState.action_retries?.[actionId] || 0;
                const hasRetried = currentRetryCount > baselineRetryCount;

                if (hasRetried) {
                    stopRetryPolling();

                    logMessages = [];
                    logOutput = "";
                    results = {};
                    completedActions = [];
                    failedActionIndex = -1;
                    currentActionIndex = -1;
                    showApproval = false;
                    approvalID = null;
                    hasReceivedMessages = false;
                    manuallyClosed = false;

                    updateStatusFromSummary(currentState);
                    connectSSE();
                    startStatusPolling();

                    isRetrying = false;

                    showSuccess("Execution Started", "Execution has started successfully");
                }
            } catch (error) {
                console.error("Retry polling error:", error);
            }
        }, 2000);
    };

    const stopRetryPolling = () => {
        if (retryPollingInterval) {
            clearInterval(retryPollingInterval);
            retryPollingInterval = null;
        }
    };

    let scheduledTime = $state("");

    // Initialize component
    onMount(() => {
        if (data.executionSummary) {
            updateStatusFromSummary(data.executionSummary);
            const execStartTime = getStartTime(data.executionSummary);
            startTime = formatDateTime(execStartTime);
            if (data.executionSummary.scheduled_at) {
                scheduledTime = formatDateTime(data.executionSummary.scheduled_at);
            }
            flowName =
                data.executionSummary.flow_name ||
                data.flowMeta?.meta?.name ||
                "";
        } else {
            flowName = data.flowMeta?.meta?.name || "";
            startTime = formatDateTime(new Date().toISOString());
        }

        // Set default selected action (first action or current running action)
        if (actions.length > 0) {
            if (currentActionIndex !== -1 && actions[currentActionIndex]) {
                selectedActionId = actions[currentActionIndex].id;
            } else {
                selectedActionId = actions[0].id;
            }
        }

        if (!eventSource) {
            connectSSE();
        }
        startStatusPolling();
        loadArtifacts();
    });

    // Auto-select running action when it changes
    $effect(() => {
        if (currentActionIndex !== -1 && actions[currentActionIndex]) {
            selectedActionId = actions[currentActionIndex].id;
        }
    });

    onDestroy(() => {
        if (eventSource) {
            eventSource.close();
        }
        stopStatusPolling();
        stopRetryPolling();
        if (rafId !== null) {
            cancelAnimationFrame(rafId);
            rafId = null;
        }
        flushMessageBuffer();
    });
</script>

<svelte:head>
    <title>Flow Execution - {flowName || "Loading..."}</title>
</svelte:head>

<div class="results-layout">
    <div class="results-main">
        <Header
            breadcrumbs={[
                { label: "Flows", url: `/view/${encodeURIComponent(namespace)}/flows` },
                {
                    label: flowName || "Loading...",
                    url: flowName
                        ? `/view/${encodeURIComponent(namespace)}/flows/${flowId}`
                        : undefined,
                },
                { label: "Execution Status" },
            ]}
            actions={[
                ...(status === "running"
                    ? [
                          {
                              label: "Stop",
                              onClick: stopFlow,
                              variant: "danger" as const,
                              icon: IconPlayerStop,
                          },
                      ]
                    : []),
                ...((status === "errored" || status === "cancelled") && !flowHasAutoRetry
                    ? [
                          {
                              label: isRetrying ? "Retrying..." : "Retry",
                              onClick: handleRetry,
                              variant: "primary" as const,
                              icon: IconRefresh,
                              tooltip: "Retry execution from the failed action",
                          },
                      ]
                    : []),
                {
                    label: "Rerun",
                    onClick: handleRerun,
                    variant: "secondary" as const,
                    icon: IconRepeat,
                    tooltip: "Create a new execution with same inputs"
                },
            ]}
        >
            {#snippet children()}
                <div class="hstack gap-2 items-center">
                    <span class="text-lighter text-sm">Status:</span>
                    <StatusBadge value={status} />
                </div>
            {/snippet}
        </Header>

        <!-- Compact info bar -->
        <div class="info-bar hstack nowrap justify-between text-sm">
            <div class="hstack nowrap gap-4">
                <span class="font-medium">{flowName || "Loading..."}</span>
                {#if data.executionSummary?.trigger_type}
                    <span class="badge {data.executionSummary.trigger_type === 'manual' ? '' : 'success'}">{data.executionSummary.trigger_type}</span>
                {/if}
                {#if startTime}
                    <span class="text-light">Started {startTime}</span>
                {/if}
                {#if data.executionSummary?.triggered_by}
                    <span class="text-light">by <strong>{data.executionSummary.triggered_by.replace(/<.*>/, '').trim()}</strong></span>
                {/if}
            </div>
            <div class="hstack nowrap gap-2 shrink-0">
                <span class="text-light">ID</span>
                <code class="text-xs">{logId}</code>
            </div>
        </div>

        <!-- Collapsible sections -->
        {#if data.executionSummary?.input || Object.keys(results).length > 0 || artifacts.length > 0}
            <div class="collapsible-sections">
                {#if data.executionSummary?.input}
                    <details>
                        <summary>Inputs</summary>
                        <pre><code>{JSON.stringify(data.executionSummary.input, null, 2)}</code></pre>
                    </details>
                {/if}

                {#if Object.keys(results).length > 0}
                    <details open>
                        <summary>Outputs</summary>
                        <div>
                            <ExecutionOutputTable {results} />
                        </div>
                    </details>
                {/if}

                {#if artifacts.length > 0}
                    <details open>
                        <summary>Artifacts</summary>
                        <div class="artifacts-container">
                            <ul class="artifacts-list">
                                {#each artifacts as artifact}
                                    <li class="artifact-item">
                                        <a
                                            href="/api/v1/{encodeURIComponent(namespace)}/flows/executions/{logId}/artifacts/download?path={encodeURIComponent(artifact.path)}"
                                            download={artifact.name}
                                            class="artifact-link"
                                        >
                                            <span class="artifact-name">{artifact.path}</span>
                                        </a>
                                        <span class="artifact-size">({(artifact.size / 1024).toFixed(2)} KB)</span>
                                    </li>
                                {/each}
                            </ul>
                        </div>
                    </details>
                {/if}
            </div>
        {/if}

        <!-- Split Panel: Actions + Terminal (fills remaining height) -->
        <div class="split-panel">
            <div class="panel-left">
                <ActionsList
                    actions={actionsList}
                    bind:selectedActionId
                    onActionSelect={handleActionSelect}
                />
            </div>

            <div class="panel-right">
                <div class="card logs-card" style="padding: 0;">
                    <div class="logs-header hstack justify-between">
                        <h5>
                            {#if selectedActionId}
                                {actionsList.find((a) => a.id === selectedActionId)?.name || "Action Logs"}
                            {:else}
                                Action Logs
                            {/if}
                        </h5>
                    </div>
                    <div class="logs-body">
                        <LogsView
                            bind:logs={logOutput}
                            {logMessages}
                            isRunning={status === "running"}
                            height="h-full"
                            theme="dark"
                            autoScroll={true}
                            showCursor={true}
                            filterByActionId={selectedActionId}
                            {logId}
                            {namespace}
                        />
                    </div>
                </div>
            </div>
        </div>
    </div>
</div>

<style>
    .results-layout {
        display: flex;
        height: 100%;
    }

    .results-main {
        flex: 1;
        display: flex;
        flex-direction: column;
        overflow: hidden;
    }

    .info-bar {
        padding: var(--space-2) var(--space-4);
        border-bottom: 1px solid var(--border);
        background: var(--card);
        flex-shrink: 0;
    }

    .collapsible-sections {
        flex-shrink: 0;
        padding: var(--space-2) var(--space-3);
    }

    .collapsible-sections details {
        margin: 0;
        background: var(--card);
    }

    .collapsible-sections summary {
        padding: var(--space-2) var(--space-3);
        font-size: var(--text-7);
        color: var(--foreground);
        font-weight: var(--font-semibold);
    }

    .split-panel {
        flex: 1;
        display: grid;
        grid-template-columns: 280px 1fr;
        min-height: 0;
        margin: var(--space-2) var(--space-3);
        border: 1px solid var(--border);
        border-radius: var(--radius-medium);
        overflow: hidden;
    }

    @media (max-width: 768px) {
        .split-panel {
            grid-template-columns: 1fr;
        }
    }

    .panel-left {
        min-height: 0;
        overflow: hidden;
        border-right: 1px solid var(--border);
    }

    .panel-left :global(.actions-panel) {
        border: none;
        border-radius: 0;
        box-shadow: none;
    }

    .panel-right {
        min-height: 0;
        overflow: hidden;
    }

    .logs-card {
        height: 100%;
        display: flex;
        flex-direction: column;
        overflow: hidden;
        border: none;
        border-radius: 0;
        box-shadow: none;
    }

    .logs-header {
        padding: var(--space-2) var(--space-4);
        border-bottom: 1px solid var(--border);
        flex-shrink: 0;
    }

    .logs-header h5 {
        margin: 0;
    }

    .logs-body {
        flex: 1;
        overflow: hidden;
        padding: var(--space-2);
    }

    .artifacts-container {
        padding: var(--space-3);
    }

    .artifacts-list {
        list-style: none;
        padding: 0;
        margin: 0;
        display: flex;
        flex-direction: column;
        gap: var(--space-2);
    }

    .artifact-item {
        display: flex;
        align-items: center;
        gap: var(--space-2);
        padding: var(--space-2) var(--space-3);
        border: 1px solid var(--border);
        border-radius: var(--radius-small);
        background: var(--background);
        transition: border-color var(--transition-fast);
    }

    .artifact-item:hover {
        border-color: var(--primary);
    }

    .artifact-link {
        color: var(--primary);
        text-decoration: none;
        font-family: var(--font-mono);
        font-size: var(--text-6);
        display: flex;
        align-items: center;
        gap: var(--space-2);
    }

    .artifact-link:hover {
        text-decoration: underline;
    }

    .artifact-size {
        font-size: var(--text-7);
        color: var(--muted-foreground);
    }
</style>
