<script lang="ts">
  import { page } from '$app/state';
  import { goto } from '$app/navigation';
  import FlowInputForm from '$lib/components/flow-input/FlowInputForm.svelte';
  import Table from '$lib/components/shared/Table.svelte';
  import Header from '$lib/components/shared/Header.svelte';
  import StatusBadge from '$lib/components/shared/StatusBadge.svelte';
  import ExecutionIdCell from '$lib/components/shared/ExecutionIdCell.svelte';
  import Tabs from '$lib/components/shared/Tabs.svelte';
  import Pagination from '$lib/components/shared/Pagination.svelte';
  import ErrorMessage from '$lib/components/shared/ErrorMessage.svelte';
  import FlowHero from '$lib/components/flows/FlowHero.svelte';
  import FlowActionsSummary from '$lib/components/flow-input/FlowActionsSummary.svelte';
  import type { PageData } from './$types';
  import type { TableColumn } from '$lib/types';
  import { DEFAULT_PAGE_SIZE } from '$lib/constants';

  let { data }: { data: PageData } = $props();
  
  let activeTab = $state<'run' | 'history'>('run');
  let historyLoading = $state(false);
  let historyError = $state('');
  let flowExecutions = $state<any[]>([]);
  let historyCurrentPage = $state(1);
  let historyItemsPerPage = $state(DEFAULT_PAGE_SIZE);
  let historyTotalCount = $state(0);
  let historyPageCount = $state(0);

  let namespace = $derived(page.params.namespace);
  let flowId = $derived(page.params.flowId);


  const loadFlowHistory = async () => {
    historyLoading = true;
    historyError = '';

    try {
      const response = await fetch(`/api/v1/${namespace}/flows/${flowId}/executions?page=${historyCurrentPage}&count_per_page=${historyItemsPerPage}`);
      const result = await response.json();

      if (!response.ok) {
        historyError = result.error || 'Failed to fetch execution history';
        flowExecutions = [];
        return;
      }

      flowExecutions = result.executions || [];
      historyTotalCount = result.total_count || 0;
      historyPageCount = result.page_count || 1;
    } catch (error) {
      console.error('Error loading flow history:', error);
      historyError = 'Failed to load execution history';
      flowExecutions = [];
    } finally {
      historyLoading = false;
    }
  };

  const goToHistoryPage = (pageNum: number) => {
    if (pageNum < 1 || pageNum > historyPageCount) return;
    historyCurrentPage = pageNum;
    loadFlowHistory();
  };

  const handleHistoryPageChange = (event: CustomEvent<{ page: number }>) => {
    goToHistoryPage(event.detail.page);
  };

  const viewExecution = (executionId: string) => {
    goto(`/view/${namespace}/results/${flowId}/${executionId}`)
  };

  const formatDateTime = (dateString: string) => {
    if (!dateString) return 'Unknown';
    const date = new Date(dateString);
    return date.toLocaleString();
  };

  const formatDuration = (startedAt: string, completedAt?: string) => {
    if (!startedAt) return 'Unknown';
    if (!completedAt) return 'Running...';

    const start = new Date(startedAt);
    const end = new Date(completedAt);
    const durationMs = end.getTime() - start.getTime();

    if (durationMs < 1000) return '<1s';

    const seconds = Math.floor(durationMs / 1000);
    const minutes = Math.floor(seconds / 60);
    const hours = Math.floor(minutes / 60);

    if (hours > 0) {
      return `${hours}h ${minutes % 60}m`;
    } else if (minutes > 0) {
      return `${minutes}m ${seconds % 60}s`;
    } else {
      return `${seconds}s`;
    }
  };

  // Watch for tab changes and load history when needed
  $effect(() => {
    if (activeTab === 'history') {
      loadFlowHistory();
    }
  });

  // Table configuration
  const tableColumns: TableColumn<any>[] = [
    {
      key: 'id',
      header: 'Execution ID',
      width: 'w-48',
      component: ExecutionIdCell
    },
    {
      key: 'status',
      header: 'Status',
      component: StatusBadge
    },
    {
      key: 'triggered_by',
      header: 'Triggered By',
      width: 'w-32',
      render: (value) => value || 'System'
    },
    {
      key: 'started_at',
      header: 'Started At',
      width: 'w-40',
      render: (value) => formatDateTime(value)
    },
    {
      key: 'duration',
      header: 'Duration',
      render: (value, row) => value || formatDuration(row.started_at, row.completed_at)
    }
  ];

  // Tab configuration
  const tabs = [
    { id: 'run', label: 'Run Flow' },
    { id: 'history', label: 'History' }
  ];
</script>

<svelte:head>
  <title>Run Flow - {data.flowMeta?.meta?.name || 'Loading...'}</title>
</svelte:head>

<Header 
  breadcrumbs={[`${page.params.namespace}`, 'Flows', data.flowMeta?.meta?.name || 'Loading...']} 
  actions={[
    { label: 'Back to Flows', onClick: () => goto(`/view/${namespace}/flows`), variant: 'secondary' }
  ]}
/>

<FlowHero 
  name={data.flowMeta?.meta?.name || 'Loading...'} 
  description={data.flowMeta?.meta?.description || 'Loading flow description...'} 
/>

<div class="bg-white border-b border-gray-200 px-6">
  <div class="max-w-4xl mx-auto">
    <Tabs {tabs} bind:activeTab />
  </div>
</div>

<!-- Tab Content -->
<div class="px-6 py-8 bg-gray-50">
  {#if activeTab === 'run'}
    <div class="max-w-2xl mx-auto">
      <FlowInputForm inputs={data.flowInputs || []} namespace={namespace!} flowId={flowId!} />
      <FlowActionsSummary actions={data.flowMeta?.actions || []} />
    </div>
  {/if}

  <!-- History Tab -->
  {#if activeTab === 'history'}
    <div class="max-w-6xl mx-auto">
      {#if historyError}
        <ErrorMessage message={historyError} />
      {/if}

      <Table
        columns={tableColumns}
        data={flowExecutions}
        onRowClick={viewExecution}
        loading={historyLoading}
        title="Execution History for {data.flowMeta?.meta?.name || 'Flow'}"
        subtitle="Past executions of this flow"
        emptyMessage="No execution history"
      />

      {#if historyPageCount > 1}
        <div class="flex items-center justify-between mt-6">
          <div class="text-sm text-gray-700">
            Showing {((historyCurrentPage - 1) * historyItemsPerPage) + 1} to {Math.min(historyCurrentPage * historyItemsPerPage, historyTotalCount)} of {historyTotalCount} results
          </div>
          <Pagination 
            currentPage={historyCurrentPage}
            totalPages={historyPageCount}
            loading={historyLoading}
            on:page-change={handleHistoryPageChange}
          />
        </div>
      {/if}
    </div>
  {/if}
</div>