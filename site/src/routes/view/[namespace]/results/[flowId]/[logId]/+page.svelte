<script lang="ts">
    import { onMount, onDestroy } from "svelte";
    import { goto } from "$app/navigation";
    import Header from "$lib/components/shared/Header.svelte";
    import StatusBadge from "$lib/components/shared/StatusBadge.svelte";
    import Tabs from "$lib/components/shared/Tabs.svelte";
    import ActionsList from "$lib/components/flow-status/ActionsList.svelte";
    import PipelineGraph from "$lib/components/flow-status/PipelineGraph.svelte";
    import RerunFromActionModal from "$lib/components/flow-status/RerunFromActionModal.svelte";
    import LogsView from "$lib/components/flow-status/LogsView.svelte";
    import ExecutionOutputTable from "$lib/components/flow-status/ExecutionOutputTable.svelte";
    import type {
        FlowMetaResp,
        ExecutionSummary,
        ActionState,
    } from "$lib/types";
    import {
        actionLevels,
        actionStages,
        actionStatusToStepStatus,
        descendants,
        type StepStatus,
    } from "$lib/utils/dag";
    import { apiClient, ApiError } from "$lib/apiClient";
    import {
        handleInlineError,
        showInfo,
        showSuccess,
        showWarning,
    } from "$lib/utils/errorHandling";
    import { formatDateTime, formatDuration, getStartTime } from "$lib/utils";
    import {
        createLogStream,
        type LogMessage,
        type LogStream,
    } from "$lib/logStream";
    import {
        IconPlayerStop,
        IconRefresh,
        IconRepeat,
        IconCopy,
        IconAlertTriangle,
        IconDownload,
    } from "@tabler/icons-svelte";

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
    let logMessages = $state<LogMessage[]>([]);
    let results = $state<Record<string, any>>({});
    let showApproval = $state(false);
    let approvalID = $state<string | null>(null);
    let selectedActionId = $state<string>("");
    let activeTab = $state("logs");
    let showTimestamps = $state(true);
    let followLogs = $state(true);
    let now = $state(Date.now());

    let stream: LogStream | null = null;

    // Retry state
    let isRetrying = $state(false);
    let rerunTargetId = $state<string | null>(null);
    let retryPollingInterval: ReturnType<typeof setInterval> | null = null;

    const resetLogState = () => {
        logMessages = [];
        results = {};
        showApproval = false;
        approvalID = null;
    };

    // Polling for execution status updates
    let statusPollingInterval: NodeJS.Timeout | null = null;
    let elapsedInterval: ReturnType<typeof setInterval> | null = null;

    // Derived values
    let namespace = $derived(data.namespace);
    let flowId = $derived(data.flowId);
    let logId = $derived(data.logId);
    let actions = $derived(data.flowMeta?.actions || []);
    let flowHasAutoRetry = $derived((data.flowMeta?.meta?.max_retries ?? 0) > 0);
    let isDAG = $derived(data.flowMeta?.meta?.execution_mode === "dag");
    let canRerunFromAction = $derived(
        (status === "completed" || status === "errored" || status === "cancelled")
        && !isRetrying,
    );
    let rerunTarget = $derived(
        actions.find((action) => action.id === rerunTargetId),
    );
    let rerunAffectedActions = $derived.by(() => {
        if (!rerunTargetId) return [];
        if (!isDAG) {
            const index = actions.findIndex((action) => action.id === rerunTargetId);
            return index === -1 ? [] : actions.slice(index);
        }
        const downstream = descendants(actions, rerunTargetId);
        downstream.add(rerunTargetId);
        return actions.filter((action) => downstream.has(action.id));
    });
    let flowName = $derived(
        data.executionSummary?.flow_name || data.flowMeta?.meta?.name || "",
    );
    let startedAt = $derived(
        data.executionSummary ? getStartTime(data.executionSummary) : undefined,
    );
    let startTime = $derived(formatDateTime(startedAt, "—"));

    // Per-action state from the server. Executions that predate this being recorded fall back to
    // inferring progress from the position of current_action_id in the list.
    let actionStates = $state<Record<string, ActionState>>({});
    let hasActionStates = $derived(Object.keys(actionStates).length > 0);

    const durationForState = (state?: ActionState) => {
        if (!state?.started_at) return "";
        const finishedAt = state.finished_at
            ?? (state.status === "running" ? new Date(now).toISOString() : undefined);
        if (!finishedAt) return "";
        return formatDuration(state.started_at, finishedAt, "");
    };

    let actionDurations = $derived.by(() => {
        const durations: Record<string, string> = {};
        for (const action of actions) {
            const duration = durationForState(actionStates[action.id]);
            if (duration) durations[action.id] = duration;
        }
        return durations;
    });

    // Transform actions into the compact execution rail.
    let actionsList = $derived(
        actions.map((action, index) => ({
            id: action.id,
            name: action.name || `Action ${index + 1}`,
            status: getActionStatus(index),
            level: isDAG ? levelByActionId[action.id] : undefined,
            needs: action.needs ?? [],
            duration: actionDurations[action.id],
            approval: action.approval,
        })),
    );

    let activeActionId = $derived(
        hasActionStates
            ? (actions.find((a) => actionStates[a.id]?.status === "running")?.id ??
                  "")
            : currentActionIndex !== -1
              ? (actions[currentActionIndex]?.id ?? "")
              : "",
    );

    let levelByActionId = $derived.by(() => {
        const map: Record<string, number> = {};
        actionLevels(actions).forEach((level, index) => {
            for (const action of level) map[action.id] = index;
        });
        return map;
    });

    let graphStatuses = $derived.by(() => {
        const map: Record<string, StepStatus> = {};
        for (const action of actionsList) map[action.id] = action.status;
        return map;
    });

    let selectedAction = $derived(
        actionsList.find((action) => action.id === selectedActionId),
    );
    let completedCount = $derived(
        actionsList.filter((action) => action.status === "completed").length,
    );
    let stageCount = $derived(actionStages(actions, isDAG ? "dag" : "sequential").length);
    let outputCount = $derived(Object.keys(results).length);
    let workspaceTabs = $derived([
        { id: "logs", label: "Logs" },
        { id: "pipeline", label: "Pipeline" },
        { id: "output", label: "Outputs", badge: outputCount },
        { id: "input", label: "Inputs" },
    ]);
    let selectedLogLineCount = $derived(
        logMessages
            .filter((message) =>
                message.action_id === selectedActionId && message.message_type === "log"
            )
            .reduce(
                (count, message) =>
                    count + message.value.split("\n").filter((line) => line.trim()).length,
                0,
            ),
    );
    let elapsed = $derived.by(() => {
        if (!startedAt) return "—";
        const completedAt = status === "running" || status === "awaiting_approval"
            ? new Date(now).toISOString()
            : data.executionSummary?.completed_at;
        return formatDuration(startedAt, completedAt, "—");
    });

    const updateExecutionStatus = async (allowStreamReconnect = true) => {
        if (!logId) return;
        try {
            const executionSummary = await apiClient.executions.getById(
                namespace,
                logId,
            );
            updateStatusFromSummary(executionSummary, allowStreamReconnect);
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

    const updateStatusFromSummary = (
        executionSummary: ExecutionSummary,
        allowStreamReconnect = true,
    ) => {
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
        actionStates = executionSummary.action_states ?? {};
        if (!hasActionStates && executionSummary.current_action_id) {
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
        } else {
            startStatusPolling();
        }

        if (allowStreamReconnect && newStatus === "running" && !stream) {
            resetLogState();
            connectLogStream();
        }
    };

    const connectLogStream = () => {
        stream?.close();
        let nextStream: LogStream;
        nextStream = createLogStream({
            namespace,
            logId,
            onReset: resetLogState,
            onMessages: (messages) => {
                const appended: LogMessage[] = [];
                for (const message of messages) processMessage(message, appended);
                if (appended.length > 0) {
                    logMessages = logMessages.concat(appended);
                }
            },
            onEnd: async (reason) => {
                await updateExecutionStatus(false);
                const active = status === "running" || status === "awaiting_approval";
                if (reason === "timeout" && active) return true;
                if (stream === nextStream) stream = null;
                return false;
            },
            onFatal: (error) => {
                if (stream === nextStream) stream = null;
                handleInlineError(error, "Unable to Stream Logs");
            },
        });
        stream = nextStream;
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

    const processMessage = (msg: LogMessage, appended: LogMessage[]) => {
        // With action states the server reports progress directly. Inferring it from the order
        // messages arrive in is only correct for a single sequential run.
        if (msg.action_id && !hasActionStates) {
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
                appended.push(msg);
                break;
            case "result":
                results = { ...results, ...(msg.results || {}) };
                break;
            case "error":
                handleInlineError(
                    new ApiError(500, "Flow execution failed", {
                        error: msg.value || "An error occurred.",
                        code: "OPERATION_FAILED",
                    }),
                    "Flow Execution Error",
                );
                status = "errored";
                if (currentActionIndex !== -1) {
                    failedActionIndex = currentActionIndex;
                }
                stopStatusPolling();
                break;
            case "approval":
                approvalID = msg.value;
                showApproval = true;
                status = "awaiting_approval";
                stopStatusPolling();
                break;
            case "cancelled":
                status = "cancelled";
                appended.push({
                    ...msg,
                    value: msg.value || "Flow execution was cancelled",
                });
                stopStatusPolling();
                break;
        }
    };

    const getActionStatus = (index: number): StepStatus => {
        const state = actionStates[actions[index]?.id];
        if (state) return actionStatusToStepStatus(state.status);

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

    const handleActionSelect = (actionId: string) => {
        selectedActionId = actionId;
        activeTab = "logs";
    };

    const copyExecutionId = async () => {
        try {
            await navigator.clipboard.writeText(logId);
            showSuccess("Copied", "Execution ID copied to clipboard");
        } catch (error) {
            handleInlineError(error, "Unable to Copy Execution ID");
        }
    };

    const downloadLogs = () => {
        const link = document.createElement("a");
        link.href = `/api/v1/${encodeURIComponent(namespace)}/logs/${logId}/download`;
        link.download = `${logId}.log`;
        document.body.appendChild(link);
        link.click();
        link.remove();
    };

    const stopFlow = async () => {
        try {
            await apiClient.executions.cancel(namespace, logId);

            status = "cancelled";

            stream?.close();
            stream = null;

            showWarning(
                "Flow Cancellation",
                "Flow cancellation signal has been sent",
            );
        } catch (error) {
            // The execution can finish between this page rendering the Stop button and
            // the click landing, in which case there is nothing left to cancel.
            if (error instanceof ApiError && error.status === 409) {
                await updateExecutionStatus();
                showInfo(
                    "Nothing to Cancel",
                    "This execution had already finished.",
                );
                return;
            }
            handleInlineError(error, "Unable to Cancel Flow");
        }
    };

    const handleRerun = () => {
        goto(`/view/${encodeURIComponent(namespace)}/flows/${flowId}?rerun_from=${logId}`);
    };

    const retryExecution = async (fromAction?: string): Promise<boolean> => {
        if (isRetrying) return false;

        try {
            isRetrying = true;

            stream?.close();
            stream = null;

            // Stop current status polling
            stopStatusPolling();

            // Capture current retry counts before calling retry. A dag execution can retry several
            // actions at once, so compare the total rather than one action's count.
            const preRetryState = await apiClient.executions.getById(namespace, logId);

            const resetActionIds = fromAction
                ? rerunAffectedActions.map((action) => action.id)
                : [];
            await apiClient.executions.retry(namespace, logId, fromAction);

            if (resetActionIds.length > 0) {
                actionStates = { ...actionStates };
                for (const actionId of resetActionIds) {
                    const previous = actionStates[actionId];
                    actionStates[actionId] = { ...previous, status: "pending", error: undefined };
                }
            }
            status = "running";
            showInfo(
                fromAction ? "Actions Queued" : "Execution Retry",
                fromAction ? `Re-running from ${fromAction}...` : "Retrying execution...",
            );
            startRetryPolling(totalRetries(preRetryState));
            return true;

        } catch (error) {
            isRetrying = false;
            handleInlineError(error, "Unable to Retry Execution");
            return false;
        }
    };

    const handleRetry = () => {
        void retryExecution();
    };

    const openRerunConfirmation = (actionId: string) => {
        rerunTargetId = actionId;
    };

    const confirmActionRerun = async () => {
        if (!rerunTargetId) return;
        if (await retryExecution(rerunTargetId)) {
            rerunTargetId = null;
        }
    };

    const totalRetries = (state: ExecutionSummary) =>
        Object.values(state.action_retries ?? {}).reduce((sum, n) => sum + n, 0);

    const startRetryPolling = (baselineRetryCount: number) => {
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

                // Check if any action has been retried
                const hasRetried =
                    totalRetries(currentState) > baselineRetryCount;

                if (hasRetried) {
                    stopRetryPolling();

                    logMessages = [];
                    results = {};
                    completedActions = [];
                    failedActionIndex = -1;
                    currentActionIndex = -1;
                    actionStates = {};
                    showApproval = false;
                    approvalID = null;
                    updateStatusFromSummary(currentState, false);
                    connectLogStream();
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

    // Initialize component
    onMount(() => {
        if (data.executionSummary) {
            updateStatusFromSummary(data.executionSummary);
        }

        // Set default selected action (first action or current running action)
        if (actions.length > 0) {
            selectedActionId = activeActionId || actions[0].id;
        }

        if (!stream) {
            connectLogStream();
        }
        startStatusPolling();
        elapsedInterval = setInterval(() => (now = Date.now()), 1000);
    });

    // Auto-select running action when it changes. Depending on the id rather than the whole state
    // map keeps a poll that changed nothing from pulling the user off the action they picked.
    $effect(() => {
        if (activeActionId) {
            selectedActionId = activeActionId;
        }
    });

    onDestroy(() => {
        stream?.close();
        stream = null;
        stopStatusPolling();
        stopRetryPolling();
        if (elapsedInterval) clearInterval(elapsedInterval);
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
                    ? [{ label: "Stop", onClick: stopFlow, variant: "danger" as const, icon: IconPlayerStop }]
                    : []),
                ...((status === "errored" || status === "cancelled") && !flowHasAutoRetry
                    ? [{
                          label: isRetrying ? "Retrying..." : "Retry",
                          onClick: handleRetry,
                          variant: "primary" as const,
                          icon: IconRefresh,
                          tooltip: "Retry execution from the failed action",
                      }]
                    : []),
                {
                    label: "Rerun",
                    onClick: handleRerun,
                    variant: "secondary" as const,
                    icon: IconRepeat,
                    tooltip: "Create a new execution with the same inputs",
                },
            ]}
        />

        <section class="execution-bar" aria-label="Execution summary">
            <StatusBadge value={status === "awaiting_approval" ? "pending_approval" : status} />
            <h1>{flowName || "Loading..."}</h1>

            <div class="progress-ribbon" aria-label={`${completedCount} of ${actions.length} actions complete`}>
                {#each actionsList as action (action.id)}
                    <span class="ribbon-{action.status}" title={`${action.name}: ${action.status.replace('_', ' ')}`}></span>
                {/each}
            </div>

            <dl class="execution-meta">
                <div>
                    <dt>Progress</dt>
                    <dd>{completedCount} / {actions.length} actions</dd>
                </div>
                <div>
                    <dt>Elapsed</dt>
                    <dd>{elapsed}</dd>
                </div>
                <div>
                    <dt>Started</dt>
                    <dd>{startTime || "—"}</dd>
                </div>
                <div>
                    <dt>Trigger</dt>
                    <dd>
                        {data.executionSummary?.trigger_type || "Unknown"}
                        {#if data.executionSummary?.triggered_by}
                            · {data.executionSummary.triggered_by.replace(/<.*>/, "").trim()}
                        {/if}
                    </dd>
                </div>
            </dl>

            <div class="execution-id">
                <code title={logId}>{logId.length > 14 ? `${logId.slice(0, 8)}…${logId.slice(-4)}` : logId}</code>
                <button class="ghost icon small" type="button" title="Copy execution ID" onclick={copyExecutionId}>
                    <IconCopy size={15} />
                </button>
            </div>
        </section>

        <div class="workspace">
            <div class="action-rail">
                <ActionsList
                    actions={actionsList}
                    bind:selectedActionId
                    onActionSelect={handleActionSelect}
                    canRerun={canRerunFromAction}
                    onRerun={openRerunConfirmation}
                />
            </div>

            <section class="canvas" aria-label="Execution details">
                <div class="canvas-header">
                    <Tabs tabs={workspaceTabs} bind:activeTab />

                    {#if activeTab === "logs"}
                        <div class="canvas-tools">
                            <label><input type="checkbox" bind:checked={showTimestamps} /> Timestamps</label>
                            <label><input type="checkbox" bind:checked={followLogs} /> Follow</label>
                            {#if status !== "running"}
                                <button class="ghost small" type="button" onclick={downloadLogs}>
                                    <IconDownload size={14} /> Download
                                </button>
                            {/if}
                        </div>
                    {/if}
                </div>

                {#if activeTab === "logs"}
                    <div role="tabpanel" class="workspace-panel logs-panel">
                        <div class="action-context">
                            <strong>{selectedAction?.name || "Action logs"}</strong>
                            {#if selectedActionId}
                                <span class="separator">·</span>
                                <span class="fact">{actions.find((action) => action.id === selectedActionId)?.executor || "no executor"}</span>
                                {#if selectedAction?.duration}
                                    <span class="separator">·</span>
                                    <span class="fact">{selectedAction.duration}</span>
                                {/if}
                            {/if}
                            <span class="fact line-count">{selectedLogLineCount} lines</span>
                        </div>

                        {#if selectedActionId && actionStates[selectedActionId]?.error}
                            <div class="action-error" role="alert" data-variant="danger">
                                <IconAlertTriangle size={16} />
                                <span>{actionStates[selectedActionId].error}</span>
                            </div>
                        {/if}

                        <div class="logs-body">
                            <LogsView
                                {logMessages}
                                isRunning={status === "running"}
                                theme="dark"
                                bind:autoScroll={followLogs}
                                bind:showTimestamp={showTimestamps}
                                showControls={false}
                                filterByActionId={selectedActionId}
                                {logId}
                                {namespace}
                            />
                        </div>
                    </div>
                {:else if activeTab === "pipeline"}
                    <div role="tabpanel" class="workspace-panel pipeline-panel">
                        <div class="graph-body">
                            <PipelineGraph
                                actions={actions}
                                statuses={graphStatuses}
                                retries={data.executionSummary?.action_retries ?? {}}
                                durations={actionDurations}
                                executionMode={isDAG ? "dag" : "sequential"}
                                bind:selectedActionId
                                onActionSelect={handleActionSelect}
                                canRerun={canRerunFromAction}
                                onRerun={openRerunConfirmation}
                            />
                        </div>
                    </div>
                {:else if activeTab === "output"}
                    <div role="tabpanel" class="workspace-panel data-panel">
                        {#if outputCount > 0}
                            <ExecutionOutputTable {results} />
                        {:else}
                            <div class="empty-state text-lighter">No outputs have been produced.</div>
                        {/if}
                    </div>
                {:else}
                    <div role="tabpanel" class="workspace-panel data-panel">
                        {#if data.executionSummary?.input}
                            <pre><code>{JSON.stringify(data.executionSummary.input, null, 2)}</code></pre>
                        {:else}
                            <div class="empty-state text-lighter">This execution has no inputs.</div>
                        {/if}
                    </div>
                {/if}
            </section>
        </div>
    </div>
</div>

{#if rerunTargetId && rerunTarget}
    <RerunFromActionModal
        target={rerunTarget}
        affectedActions={rerunAffectedActions}
        onConfirm={confirmActionRerun}
        onClose={() => (rerunTargetId = null)}
    />
{/if}

<style>
    :global(main.app-content:has(.results-layout)) {
        overflow: hidden;
        background: var(--background);
    }
    :global(main.app-content:has(.results-layout) > .app-footer) { display: none; }

    .results-layout,
    .results-main {
        display: flex;
        flex: 1 1 auto;
        min-height: 0;
        width: 100%;
    }
    .results-main {
        flex-direction: column;
        background: var(--card);
    }

    .execution-bar {
        display: flex;
        align-items: center;
        gap: var(--space-4);
        flex-shrink: 0;
        min-width: 0;
        padding: var(--space-3) var(--space-6);
        background: var(--background);
        border-bottom: 1px solid var(--border);
    }
    .execution-bar h1 {
        max-width: 22rem;
        margin: 0;
        overflow: hidden;
        font-size: var(--text-5);
        text-overflow: ellipsis;
        white-space: nowrap;
    }
    .progress-ribbon {
        display: flex;
        align-items: center;
        gap: 2px;
        flex-shrink: 0;
    }
    .progress-ribbon span {
        width: 0.6rem;
        height: 1.1rem;
        border-radius: var(--radius-small);
        background: var(--faint);
    }
    .progress-ribbon .ribbon-completed { background: var(--success); }
    .progress-ribbon .ribbon-failed { background: var(--danger); }
    .progress-ribbon .ribbon-running { background: var(--primary); animation: pulse 2s infinite; }
    .progress-ribbon .ribbon-awaiting_approval { background: var(--warning); }
    .progress-ribbon .ribbon-cancelled,
    .progress-ribbon .ribbon-skipped { background: var(--faint-foreground); }

    .execution-meta {
        display: flex;
        align-items: center;
        gap: var(--space-6);
        min-width: 0;
        margin: 0;
        padding-inline-start: var(--space-2);
        overflow: hidden;
        border-inline-start: 1px solid var(--border);
    }
    .execution-meta > div { min-width: 0; }
    .execution-meta dt {
        color: var(--muted-foreground);
        font-size: var(--text-8);
        line-height: 1.3;
        letter-spacing: 0.04em;
        text-transform: uppercase;
    }
    .execution-meta dd {
        margin: 0;
        overflow: hidden;
        font-size: var(--text-7);
        font-weight: var(--font-medium);
        line-height: 1.3;
        text-overflow: ellipsis;
        white-space: nowrap;
    }
    .execution-id {
        display: flex;
        align-items: center;
        gap: var(--space-1);
        flex-shrink: 0;
        margin-inline-start: auto;
    }
    .execution-id code {
        color: var(--muted-foreground);
        font-size: var(--text-8);
    }

    .workspace {
        display: grid;
        grid-template-columns: 17rem minmax(0, 1fr);
        flex: 1;
        min-height: 0;
        background: var(--card);
    }
    .action-rail {
        min-height: 0;
        overflow: hidden;
        border-inline-end: 1px solid var(--border);
    }
    .canvas {
        display: flex;
        flex-direction: column;
        min-width: 0;
        min-height: 0;
        background: var(--card);
    }
    .canvas-header {
        display: flex;
        align-items: center;
        flex-shrink: 0;
        height: var(--space-12);
        padding-inline: var(--space-4);
        background: var(--card);
        border-bottom: 1px solid var(--border);
    }
    .canvas-tools {
        display: flex;
        align-items: center;
        gap: var(--space-3);
        margin-inline-start: auto;
        color: var(--muted-foreground);
    }
    .canvas-tools label {
        margin: 0;
        font-size: var(--text-8);
        font-weight: var(--font-normal);
    }
    .workspace-panel {
        display: flex;
        flex: 1;
        flex-direction: column;
        min-height: 0;
        padding: 0;
    }
    .action-context {
        display: flex;
        align-items: center;
        gap: var(--space-3);
        flex-shrink: 0;
        min-width: 0;
        padding: var(--space-2) var(--space-4);
        overflow: hidden;
        background: var(--card);
        border-bottom: 1px solid var(--border);
        font-size: var(--text-7);
        white-space: nowrap;
    }
    .action-context strong {
        overflow: hidden;
        text-overflow: ellipsis;
    }
    .action-context .fact { color: var(--muted-foreground); }
    .separator { color: var(--border); }
    .line-count,
    .graph-help { margin-inline-start: auto; }
    .action-error {
        flex-shrink: 0;
        margin: 0;
        padding: var(--space-3) var(--space-4);
        border: none;
        border-bottom: 1px solid color-mix(in srgb, var(--danger) 30%, transparent);
        border-radius: 0;
    }
    .logs-body,
    .graph-body {
        flex: 1;
        min-height: 0;
        overflow: hidden;
    }
    .logs-body :global(.log-container) { border-radius: 0; }
    .data-panel {
        overflow: auto;
        padding: var(--space-4);
    }
    .data-panel pre { margin: 0; }
    .empty-state {
        display: grid;
        flex: 1;
        place-items: center;
        font-size: var(--text-7);
    }

    @media (max-width: 1100px) {
        .execution-meta { gap: var(--space-3); }
        .execution-meta > div:nth-child(3) { display: none; }
    }

    @keyframes pulse {
        0%, 100% { opacity: 1; }
        50% { opacity: 0.5; }
    }
</style>
