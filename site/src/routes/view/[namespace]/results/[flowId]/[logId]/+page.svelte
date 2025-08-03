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
  import type { PageData } from './$types';
  import type { FlowMetaResp, ExecutionSummary } from '$lib/types';

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
  let status = $state<'running' | 'completed' | 'awaiting_approval' | 'errored'>('running');
  let currentActionIndex = $state(-1);
  let completedActions = $state<number[]>([]);
  let failedActionIndex = $state(-1);
  let logOutput = $state('');
  let results = $state<Record<string, any>>({});
  let showApproval = $state(false);
  let approvalID = $state<string | null>(null);
  let errorMessage = $state<string | null>(null);
  let activeTab = $state('logs');
  let startTime = $state('');
  let flowName = $state('');

  // WebSocket connection
  let ws: WebSocket | null = null;
  let hasReceivedMessages = $state(false);

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
        console.error('Failed to parse WebSocket message:', e);
      }
      processMessage(msg);
    };

    ws.onclose = (event) => {
      console.log('WebSocket closed:', event);
      if (event.code === 1000) {
        status = 'completed';
        for (let i = 0; i < actions.length; i++) {
          if (!completedActions.includes(i)) {
            completedActions.push(i);
          }
        }
      } else if (event.reason) {
        errorMessage = event.reason;
        status = 'errored';
      }
    };

    ws.onerror = (error) => {
      console.error('WebSocket error:', error);
    };
  };

  const reconstructProgress = (currentActionId: string, executionStatus: string) => {
    let currentActionIndex = actions.findIndex(action => action.id === currentActionId);
    if (currentActionIndex === -1) return;

    for (let i = 0; i < currentActionIndex; i++) {
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
      failedActionIndex = currentActionIndex;
      currentActionIndex = -1;
    } else if (executionStatus === 'running' || executionStatus === 'pending') {
      currentActionIndex = currentActionIndex;
    } else if (executionStatus === 'pending_approval') {
      currentActionIndex = currentActionIndex;
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
        break;
      case 'result':
        results = { ...results, ...(msg.results || {}) };
        if (currentActionIndex !== -1 && !completedActions.includes(currentActionIndex)) {
          completedActions.push(currentActionIndex);
        }
        break;
      case 'error':
        errorMessage = msg.value || "An error occurred.";
        status = 'errored';
        if (currentActionIndex !== -1) {
          failedActionIndex = currentActionIndex;
        }
        break;
      case 'approval':
        approvalID = msg.value;
        showApproval = true;
        status = 'awaiting_approval';
        break;
      default:
        logOutput += (msg.value || '') + '\n';
    }
  };

  const goBack = () => {
    goto(`/view/${namespace}/flows`);
  };

  const getActionStatus = (index: number): 'pending' | 'running' | 'completed' | 'failed' => {
    if (index === failedActionIndex) return 'failed';
    if (completedActions.includes(index)) return 'completed';
    if (index === currentActionIndex && status === 'running') return 'running';
    return 'pending';
  };

  const dismissError = () => {
    errorMessage = null;
  };

  const dismissApproval = () => {
    showApproval = false;
  };

  // Initialize component
  onMount(() => {
    if (data.executionSummary) {
      const execStatus = data.executionSummary.status;
      status = execStatus === 'pending' ? 'running' : 
               execStatus === 'pending_approval' ? 'awaiting_approval' : 
               execStatus;
      startTime = new Date(data.executionSummary.started_at).toLocaleString();
      flowName = data.executionSummary.flow_name || data.flowMeta?.meta?.name || '';
      
      if (data.executionSummary.current_action_id) {
        reconstructProgress(data.executionSummary.current_action_id, data.executionSummary.status);
      }
    } else {
      flowName = data.flowMeta?.meta?.name || ''
      startTime = new Date().toLocaleString();
    }

    connectWebSocket();
  });

  onDestroy(() => {
    if (ws) {
      ws.close();
    }
  });
</script>

<svelte:head>
  <title>Flow Execution - {flowName || 'Loading...'}</title>
</svelte:head>

<div class="flex h-screen bg-gray-50">
  <main class="flex-1 flex flex-col overflow-hidden">
    <Header 
      breadcrumbs={['Flows', flowName || 'Loading...', 'Execution Status']}
      actions={[
        {
          label: 'Back to Flows',
          onClick: goBack,
          variant: 'secondary'
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

        {#if errorMessage}
          <div class="mt-6">
            <Alert
              type="error"
              title="Error"
              message={errorMessage}
              dismissible={true}
              onDismiss={dismissError}
            />
          </div>
        {/if}
      </div>
    </div>
  </main>
</div>