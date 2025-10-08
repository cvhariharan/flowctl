<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { goto } from '$app/navigation';
  import Header from '$lib/components/shared/Header.svelte';
  import StatusBadge from '$lib/components/shared/StatusBadge.svelte';
  import Alert from '$lib/components/flow-status/Alert.svelte';
  import Tabs from '$lib/components/shared/Tabs.svelte';
  import PipelineProgress from '$lib/components/flow-status/PipelineProgress.svelte';
  import LogsView from '$lib/components/flow-status/LogsView.svelte';
  import FlowInfoCard from '$lib/components/flow-status/FlowInfoCard.svelte';
  import ExecutionOutputTable from '$lib/components/flow-status/ExecutionOutputTable.svelte';
  import EmptyState from '$lib/components/flow-status/EmptyState.svelte';
  import JsonDisplay from '$lib/components/shared/JsonDisplay.svelte';
  import type { PageData } from './$types';
  import type { FlowMetaResp, ExecutionSummary } from '$lib/types';
  import { apiClient, ApiError } from '$lib/apiClient';
  import { handleInlineError, showInfo, showSuccess, showWarning } from '$lib/utils/errorHandling';

  let { data }: { 
    data: {
      namespace: string;
      flowId: string; 
      logId: string;
      flowMeta?: FlowMetaResp;
      executionSummary?: ExecutionSummary;
      error?: string;
    }
  } = $props();

  // Flow execution state
  let status = $state<'running' | 'completed' | 'awaiting_approval' | 'errored' | 'cancelled'>('running');
  let currentActionIndex = $state(-1);
  let completedActions = $state<number[]>([]);
  let failedActionIndex = $state(-1);
  let logOutput = $state('');
  let logMessages = $state<Array<{action_id: string; message_type: string; value: string; timestamp: string}>>([]);
  let results = $state<Record<string, any>>({});
  let showApproval = $state(false);
  let approvalID = $state<string | null>(null);
  let activeTab = $state('logs');
  let startTime = $state('');
  let flowName = $state('');

  // WebSocket connection
  let ws: WebSocket | null = null;
  let hasReceivedMessages = $state(false);
  let manuallyClosed = $state(false);
  
  // Polling for execution status updates
  let statusPollingInterval: NodeJS.Timeout | null = null;

  // Derived values
  let namespace = $derived(data.namespace);
  let flowId = $derived(data.flowId);
  let logId = $derived(data.logId);
  let actions = $derived(data.flowMeta?.actions || []);

  // Transform actions into pipeline steps
  let pipelineSteps = $derived(
    actions.map((action, index) => ({
      id: action.id,
      name: action.name || `Step ${index + 1}`,
      status: getActionStatus(index)
    }))
  );

  // Tab configuration
  let tabs = $derived([
    { id: 'logs', label: 'Real-time Logs' },
    { 
      id: 'output', 
      label: 'Execution Output',
      badge: Object.keys(results).length > 0 ? Object.keys(results).length : undefined
    }
  ]);

  const updateExecutionStatus = async () => {
    try {
      const executionSummary = await apiClient.executions.getById(namespace, logId);
      updateStatusFromSummary(executionSummary);
    } catch (error) {
      // Silently handle errors during polling to avoid spam
      console.error('Failed to fetch execution summary:', error);
    }
  };

  const startStatusPolling = () => {
    // Always stop any existing polling first
    stopStatusPolling();
    // Poll every 2 seconds when flow is active
    if (status === 'running' || status === 'awaiting_approval') {
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
    
    if (execStatus === 'pending' || execStatus === 'running') {
      newStatus = 'running';
    } else if (execStatus === 'pending_approval') {
      newStatus = 'awaiting_approval';
      approvalID = executionSummary.current_action_id; // Use current_action_id for approval context
      showApproval = true;
    } else if (execStatus === 'cancelled') {
      newStatus = 'cancelled';
    } else if (execStatus === 'completed') {
      newStatus = 'completed';
    } else if (execStatus === 'errored') {
      newStatus = 'errored';
    } else {
      newStatus = 'running';
    }
    
    // Update status and reconstruct progress
    status = newStatus;
    if (executionSummary.current_action_id) {
      reconstructProgress(executionSummary.current_action_id, executionSummary.status);
    }
    
    // Start/stop polling based on status
    if (newStatus === 'completed' || newStatus === 'errored' || newStatus === 'cancelled') {
      stopStatusPolling();
    } else {
      startStatusPolling();
    }
  };

  const connectWebSocket = () => {
    const protocol = window.location.protocol === 'https:' ? 'wss' : 'ws';
    const wsUrl = `${protocol}://${window.location.host}/api/v1/${namespace}/logs/${logId}`;
    
    ws = new WebSocket(wsUrl);

    ws.onmessage = (event) => {
      hasReceivedMessages = true;
      let msg = {};
      try { 
        msg = JSON.parse(event.data); 
      } catch (e) {
        handleInlineError(e, 'WebSocket Message Parse Error');
      }
      processMessage(msg);
    };

    ws.onclose = (event) => {
      console.log('WebSocket closed:', event);
      // Don't update status if we manually closed the connection (e.g., during cancellation)
      if (manuallyClosed) {
        return;
      }
      
      // Let the API polling handle status updates on WebSocket close
      // This provides more reliable status information from the backend
      if (event.code === 1000) {
        updateExecutionStatus(); // Fetch final status from API
      } else if (event.reason) {
        handleInlineError(new Error(event.reason), 'WebSocket Connection Error');
        updateExecutionStatus(); // Check actual status from API
      }
    };

  };

  const reconstructProgress = (currentActionId: string, executionStatus: string) => {
    let actionIndex = actions.findIndex(action => action.id === currentActionId);
    if (actionIndex === -1) return;

    for (let i = 0; i < actionIndex; i++) {
      if (!completedActions.includes(i)) {
        completedActions.push(i);
      }
    }

    if (executionStatus === 'completed') {
      for (let i = 0; i < actions.length; i++) {
        if (!completedActions.includes(i)) {
          completedActions.push(i);
        }
      }
      currentActionIndex = -1;
    } else if (executionStatus === 'errored') {
      failedActionIndex = actionIndex;
      currentActionIndex = -1;
    } else if (executionStatus === 'cancelled') {
      failedActionIndex = actionIndex;
      currentActionIndex = -1;
      status = 'cancelled';
    } else if (executionStatus === 'running' || executionStatus === 'pending') {
      currentActionIndex = actionIndex;
    } else if (executionStatus === 'pending_approval') {
      currentActionIndex = actionIndex;
      status = 'awaiting_approval';
    }
  };

  const processMessage = (msg: any) => {
    if (msg.action_id) {
      const actionIndex = actions.findIndex(a => a.id === msg.action_id);
      if (actionIndex !== -1) {
        if (currentActionIndex !== -1 && currentActionIndex !== actionIndex) {
          if (!completedActions.includes(currentActionIndex)) {
            completedActions.push(currentActionIndex);
          }
        }
        currentActionIndex = actionIndex;
      }
    }

    switch (msg.message_type) {
      case 'log':
        logOutput += (msg.value || '') + '\n';
        logMessages.push({
          action_id: msg.action_id || '',
          message_type: msg.message_type,
          value: msg.value || '',
          timestamp: msg.timestamp || ''
        });
        break;
      case 'result':
        results = { ...results, ...(msg.results || {}) };
        if (currentActionIndex !== -1 && !completedActions.includes(currentActionIndex)) {
          completedActions.push(currentActionIndex);
        }
        break;
      case 'error':
        // Check if the error indicates cancellation
        if (msg.value && msg.value.includes('cancelled')) {
          status = 'cancelled';
        } else {
          handleInlineError(new ApiError(500, 'Flow execution failed', {
            error: msg.value || "An error occurred.",
            code: "OPERATION_FAILED"
          }), 'Flow Execution Error');
          status = 'errored';
        }
        if (currentActionIndex !== -1) {
          failedActionIndex = currentActionIndex;
        }
        break;
      case 'approval':
        approvalID = msg.value;
        showApproval = true;
        status = 'awaiting_approval';
        stopStatusPolling(); // Approval state is stable, no need to poll aggressively
        break;
      case 'cancelled':
        status = 'cancelled';
        logOutput += (msg.value || 'Flow execution was cancelled') + '\n';
        logMessages.push({
          action_id: msg.action_id || '',
          message_type: msg.message_type,
          value: msg.value || 'Flow execution was cancelled',
          timestamp: msg.timestamp || ''
        });
        stopStatusPolling(); // Flow finished, no need to continue polling
        break;
      default:
        logOutput += (msg.value || '') + '\n';
        logMessages.push({
          action_id: msg.action_id || '',
          message_type: msg.message_type,
          value: msg.value || '',
          timestamp: msg.timestamp || ''
        });
    }
  };

  const goBack = () => {
    goto(`/view/${namespace}/flows`);
  };

  const getActionStatus = (index: number): 'pending' | 'running' | 'completed' | 'failed' | 'awaiting_approval' | 'cancelled' => {
    // Handle completed actions - they should always stay green
    if (completedActions.includes(index)) return 'completed';
    
    // Handle failed action
    if (index === failedActionIndex) return 'failed';
    
    // Handle current action based on flow status
    if (index === currentActionIndex) {
      if (status === 'running') return 'running';
      if (status === 'awaiting_approval') return 'awaiting_approval';
      if (status === 'cancelled') return 'cancelled';
      if (status === 'errored') return 'failed';
    }
    
    // Special case: if flow is awaiting approval and no current action is set,
    // find the first non-completed action to mark as awaiting approval
    if (status === 'awaiting_approval' && currentActionIndex === -1) {
      const firstIncompleteIndex = actions.findIndex((_, i) => !completedActions.includes(i));
      if (index === firstIncompleteIndex) return 'awaiting_approval';
    }
    
    // Handle remaining actions based on flow status
    const lastProcessedIndex = Math.max(
      currentActionIndex >= 0 ? currentActionIndex : -1, 
      failedActionIndex >= 0 ? failedActionIndex : -1, 
      completedActions.length > 0 ? Math.max(...completedActions) : -1
    );
    
    // If flow has failed, errored, or cancelled, actions after the failure/cancellation point should be cancelled
    if ((status === 'errored' || status === 'cancelled') && index > lastProcessedIndex) {
      return 'cancelled';
    }
    
    // Default to pending for actions that haven't started yet
    return 'pending';
  };


  const dismissApproval = () => {
    showApproval = false;
  };

  const stopFlow = async () => {
    try {
      await apiClient.executions.cancel(namespace, logId);
      
      // Set status first, then close WebSocket to prevent race condition
      status = 'cancelled';
      
      // Mark as manually closed and close WebSocket connection
      manuallyClosed = true;
      if (ws) {
        ws.close();
      }
      
      showWarning('Flow Cancellation', 'Flow cancellation signal has been sent');
    } catch (error) {
      handleInlineError(error, 'Unable to Cancel Flow');
    }
  };

  // Initialize component
  onMount(() => {
    if (data.executionSummary) {
      updateStatusFromSummary(data.executionSummary);
      startTime = new Date(data.executionSummary.started_at).toLocaleString();
      flowName = data.executionSummary.flow_name || data.flowMeta?.meta?.name || '';
    } else {
      flowName = data.flowMeta?.meta?.name || ''
      startTime = new Date().toLocaleString();
    }

    connectWebSocket();
    startStatusPolling(); // Start polling for status updates
  });

  onDestroy(() => {
    if (ws) {
      ws.close();
    }
    stopStatusPolling();
  });
</script>

<svelte:head>
  <title>Flow Execution - {flowName || 'Loading...'}</title>
</svelte:head>

<div class="flex h-screen bg-gray-50">
  <main class="flex-1 flex flex-col overflow-hidden">
    <Header 
      breadcrumbs={[
        { label: 'Flows', url: `/view/${namespace}/flows` },
        { label: flowName || 'Loading...', url: flowName ? `/view/${namespace}/flows/${flowId}` : undefined },
        { label: 'Execution Status' }
      ]}
      actions={[
        ...(status === 'running' ? [{
          label: 'Stop Flow',
          onClick: stopFlow,
          variant: 'danger' as const
        }] : []),
        {
          label: 'Back to Flows',
          onClick: goBack,
          variant: 'secondary' as const
        }
      ]}
    >
      {#snippet children()}
        <div class="flex items-center gap-2">
          <span class="text-sm text-gray-500">Status:</span>
          <StatusBadge value={status} />
        </div>
      {/snippet}
    </Header>

    <!-- Page Content -->
    <div class="flex-1 overflow-y-auto p-6 bg-gray-50">
      <div class="max-w-7xl mx-auto">
        <FlowInfoCard 
          flowName={flowName || 'Loading...'}
          {startTime}
          executionId={logId}
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

        <!-- Pipeline Progress -->
        <div class="mb-6">
          <PipelineProgress 
            steps={pipelineSteps} 
            title="Pipeline Progress"
            orientation="horizontal"
            size="md"
          />
        </div>

        <!-- Tabs Content -->
        <div class="bg-white rounded-lg shadow-sm border border-gray-200 overflow-hidden">
          <Tabs {tabs} bind:activeTab />

          <!-- Tab Content -->
          <div class="p-6">
            {#if activeTab === 'logs'}
              <LogsView
                bind:logs={logOutput}
                logMessages={logMessages}
                isRunning={status === 'running'}
                height="h-96"
                theme="dark"
                autoScroll={true}
                showCursor={true}
              />
            {:else if activeTab === 'output'}
              <ExecutionOutputTable {results} />
              {#if Object.keys(results).length === 0}
                <EmptyState message="No output variables yet" />
              {/if}
            {/if}
          </div>
        </div>

        <!-- Alerts -->
        {#if showApproval}
          <div class="mt-6">
            <Alert
              type="warning"
              title="Approval Required"
              message="This flow requires manual approval to continue execution."
              dismissible={true}
              onDismiss={dismissApproval}
              actions={[
                {
                  label: "Review Request",
                  href: `/view/${namespace}/approvals/${approvalID}`,
                  primary: true
                }
              ]}
            />
          </div>
        {/if}

      </div>
    </div>
  </main>
</div>