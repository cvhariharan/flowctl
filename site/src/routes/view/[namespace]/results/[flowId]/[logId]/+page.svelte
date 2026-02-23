<script lang="ts">
    import { onMount, onDestroy } from "svelte";
    import { goto } from "$app/navigation";
    import Header from "$lib/components/shared/Header.svelte";
    import StatusBadge from "$lib/components/shared/StatusBadge.svelte";
    import ActionsList from "$lib/components/flow-status/ActionsList.svelte";
    import LogsView from "$lib/components/flow-status/LogsView.svelte";
    import FlowInfoCard from "$lib/components/flow-status/FlowInfoCard.svelte";
    import ExecutionOutputTable from "$lib/components/flow-status/ExecutionOutputTable.svelte";
    import JsonDisplay from "$lib/components/shared/JsonDisplay.svelte";
    import type { FlowMetaResp, ExecutionSummary } from "$lib/types";
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

    // Polling for execution status updates
    let statusPollingInterval: NodeJS.Timeout | null = null;

    // Derived values
    let namespace = $derived(data.namespace);
    let flowId = $derived(data.flowId);
    let logId = $derived(data.logId);
    let actions = $derived(data.flowMeta?.actions || []);

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
        } else if (executionSummary.status === "completed") {
            // Mark all actions as completed when execution finishes
            console.log("POLLING: completed, actions:", actions.length, "current completedActions:", completedActions);
            const allIndexes = actions.map((_, i) => i);
            completedActions = allIndexes;
            currentActionIndex = -1;
            status = "completed";
            console.log("POLLING: set completedActions to", completedActions);
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

        // Reconnect SSE if status changed to running but not connected (scheduler retry)
        if (newStatus === "running" && !eventSource) {
            resetLogState();
            connectSSE();
        }
    };

    const connectSSE = () => {
        const sseUrl = `/api/v1/${namespace}/logs/${logId}`;
        eventSource = new EventSource(sseUrl);

        eventSource.onmessage = (event) => {
            hasReceivedMessages = true;
            let msg = {};
            try {
                msg = JSON.parse(event.data);
            } catch (e) {
                handleInlineError(e, "SSE Message Parse Error");
            }
            console.log("SSE raw:", msg.message_type, JSON.stringify(msg).substring(0,100));
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
            // Only set awaiting_approval if not already completed
            if (status !== "completed" && status !== "errored" && status !== "cancelled") {
                currentActionIndex = actionIndex;
                status = "awaiting_approval";
            }
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
                // result means the last action completed
                completedActions = actions.map((_, i) => i);
                currentActionIndex = -1;
                status = "completed";
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
            case "approved":
                console.log("SSE: approved message received!");
                flushMessageBuffer();
                showApproval = false;
                status = "running";
                startStatusPolling();
                break;
            case "completed":
                flushMessageBuffer();
                console.log("SSE: completed received, actions:", actions.length);
                status = "completed";
                completedActions = actions.map((_, i) => i);
                currentActionIndex = -1;
                console.log("SSE: completedActions set to", completedActions);
                if (eventSource) {
                    eventSource.close();
                    eventSource = null;
                }
                stopStatusPolling();
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
        goto(`/view/${namespace}/flows`);
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
        goto(`/view/${namespace}/flows/${flowId}?rerun_from=${logId}`);
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

<div class="flex h-screen bg-muted">
    <main class="flex-1 flex flex-col overflow-hidden">
        <Header
            breadcrumbs={[
                { label: "Flows", url: `/view/${namespace}/flows` },
                {
                    label: flowName || "Loading...",
                    url: flowName
                        ? `/view/${namespace}/flows/${flowId}`
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
                ...(status === "errored" || status === "cancelled"
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
                <div class="flex items-center gap-2">
                    <span class="text-sm text-muted-foreground">Status:</span>
                    <StatusBadge value={status} />
                </div>
            {/snippet}
        </Header>

        <!-- Page Content -->
        <div class="flex-1 overflow-y-auto p-6 bg-muted">
            <div class="max-w-7xl mx-auto">
                <FlowInfoCard
                    flowName={flowName || "Loading..."}
                    {startTime}
                    executionId={logId}
                    scheduledAt={scheduledTime}
                    triggerType={data.executionSummary?.trigger_type}
                    triggeredBy={data.executionSummary?.triggered_by}
                />

                <!-- Flow Input -->
                {#if data.executionSummary?.input}
                    <div class="mb-6">
                        <JsonDisplay
                            data={data.executionSummary.input}
                            title="Inputs"
                        />
                    </div>
                {/if}

                <!-- Split Panel Layout: Actions List and Logs -->
                <div class="mb-6 grid grid-cols-12 gap-6 h-[650px]">
                    <!-- Left Panel: Actions List -->
                    <div class="col-span-12 md:col-span-4 lg:col-span-3 h-full">
                        <ActionsList
                            actions={actionsList}
                            bind:selectedActionId
                            onActionSelect={handleActionSelect}
                        />
                    </div>

                    <!-- Right Panel: Terminal / Logs -->
                    <div class="col-span-12 md:col-span-8 lg:col-span-9 h-full">
                        <div
                            class="bg-card rounded-lg border border-input h-full flex flex-col overflow-hidden"
                        >
                            <div class="px-6 py-5 border-b border-input">
                                <h2
                                    class="text-base font-semibold text-foreground"
                                >
                                    {#if selectedActionId}
                                        {actionsList.find(
                                            (a) => a.id === selectedActionId,
                                        )?.name || "Action Logs"}
                                    {:else}
                                        Action Logs
                                    {/if}
                                </h2>
                            </div>
                            <div class="flex-1 overflow-hidden p-6">
                                <div class="h-full">
                                    <LogsView
                                        bind:logs={logOutput}
                                        {logMessages}
                                        isRunning={status === "running"}
                                        height="h-full"
                                        theme="dark"
                                        autoScroll={true}
                                        showCursor={true}
                                        filterByActionId={selectedActionId}
                                    />
                                </div>
                            </div>
                        </div>
                    </div>
                </div>

                <!-- Execution Output -->
                {#if Object.keys(results).length > 0}
                    <div
                        class="mb-6 bg-card rounded-lg border border-input overflow-hidden"
                    >
                        <div class="px-6 py-5 border-b border-input">
                            <h2 class="text-base font-semibold text-foreground">
                                Execution Output
                            </h2>
                        </div>
                        <div class="p-6">
                            <ExecutionOutputTable {results} />
                        </div>
                    </div>
                {/if}
            </div>
        </div>
    </main>
</div>
