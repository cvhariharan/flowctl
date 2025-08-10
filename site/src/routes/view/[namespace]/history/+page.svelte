<script lang="ts">
	import { browser } from '$app/environment';
	import { page } from '$app/state';
	import type { PageData } from './$types';
	import PageHeader from '$lib/components/shared/PageHeader.svelte';
	import SearchInput from '$lib/components/shared/SearchInput.svelte';
	import Table from '$lib/components/shared/Table.svelte';
	import Pagination from '$lib/components/shared/Pagination.svelte';
	import ExecutionIdCell from '$lib/components/shared/ExecutionIdCell.svelte';
	import StatusBadge from '$lib/components/shared/StatusBadge.svelte';
	import { apiClient } from '$lib/apiClient';
	import type { ExecutionSummary } from '$lib/types';
	import { DEFAULT_PAGE_SIZE } from '$lib/constants';
	import Header from '$lib/components/shared/Header.svelte';
	import { handleInlineError, showSuccess } from '$lib/utils/errorHandling';

	let { data }: { data: PageData } = $props();

	// State
	let executions = $state(data.executions);
	let totalCount = $state(data.totalCount);
	let pageCount = $state(data.pageCount);
	let currentPage = $state(data.currentPage);
	let searchQuery = $state(data.searchQuery);
	let loading = $state(false);

	// Table configuration
	let tableColumns = [
		{
			key: 'id',
			header: 'Execution ID',
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
			render: (_value: any, execution: ExecutionSummary) => `
				<div class="flex items-center">
					<div class="w-8 h-8 rounded-full bg-blue-100 flex items-center justify-center mr-3">
						<svg class="w-4 h-4 text-blue-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"/>
						</svg>
					</div>
					<span class="text-sm font-medium text-gray-900">${execution.triggered_by || 'System'}</span>
				</div>
			`
		},
		{
			key: 'started_at',
			header: 'Started At',
			render: (_value: any, execution: ExecutionSummary) => `
				<div>
					<div>${formatDate(execution.started_at)}</div>
					<div class="text-xs text-gray-500">${formatTime(execution.started_at)}</div>
				</div>
			`
		},
		{
			key: 'duration',
			header: 'Duration',
			render: (_value: any, execution: ExecutionSummary) => 
				execution.duration || formatDuration(execution.started_at, execution.completed_at)
		}
	];

	let tableActions = [
		{
			label: 'View',
			onClick: (execution: ExecutionSummary) => viewExecution(execution.id),
			className: 'text-blue-600 hover:text-blue-800'
		},
		{
			label: 'Logs',
			onClick: (execution: ExecutionSummary) => viewLogs(execution.id),
			className: 'text-green-600 hover:text-green-800'
		}
	];

	// Functions
	async function fetchExecutions(filter: string = '', pageNumber: number = 1) {
		if (!browser) return;
		
		loading = true;
		try {
			const response = await apiClient.executions.list(data.namespace, {
				page: pageNumber,
				count_per_page: DEFAULT_PAGE_SIZE,
				filter: filter
			});

			executions = response.executions || [];
			totalCount = response.total_count || 0;
			pageCount = response.page_count || 1;
		} catch (error) {
			handleInlineError(error, 'Unable to Load Execution History');
		} finally {
			loading = false;
		}
	}

	function handleSearch(query: string) {
		searchQuery = query;
		fetchExecutions(query);
	}

	function handlePageChange(event: CustomEvent<{ page: number }>) {
		currentPage = event.detail.page;
		fetchExecutions('', currentPage);
	}

	function viewExecution(executionId: string) {
		window.location.href = `/api/v1/${data.namespace}/flows/executions/${executionId}`;
	}

	function viewLogs(executionId: string) {
		const logUrl = `/view/${data.namespace}/logs/${executionId}`;
		window.open(logUrl, '_blank');
	}

	function formatDate(dateString: string): string {
		if (!dateString) return 'Unknown';
		const date = new Date(dateString);
		const now = new Date();
		const diffMs = now.getTime() - date.getTime();
		const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));

		if (diffDays === 0) return 'Today';
		if (diffDays === 1) return 'Yesterday';
		if (diffDays < 7) return `${diffDays} days ago`;
		return date.toLocaleDateString();
	}

	function formatTime(dateString: string): string {
		if (!dateString) return '';
		const date = new Date(dateString);
		return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
	}

	function formatDuration(startedAt: string, completedAt: string): string {
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
	}

	function handleRowClick(execution: ExecutionSummary) {
		viewExecution(execution.id);
	}
</script>

<svelte:head>
  <title>Execution History - {page.params.namespace} - Flowctl</title>
</svelte:head>

<Header breadcrumbs={[`${page.params.namespace}`, "History"]}>
  {#snippet children()}
    <SearchInput
      bind:value={searchQuery}
      placeholder="Search executions..."
      {loading}
      onSearch={handleSearch}
    />
  {/snippet}
</Header>

<div class="p-12">
	<!-- Page Header -->
	<PageHeader 
		title="Execution History"
		subtitle="View all flow execution history across all flows in this namespace"
	/>

	<!-- Executions Table -->
	<div class="pt-6">
		<Table
			data={executions}
			columns={tableColumns}
			actions={tableActions}
			{loading}
			onRowClick={handleRowClick}
			emptyMessage="No execution history found. Executions will appear here once flows are triggered."
			emptyIcon='<svg class="w-16 h-16 text-gray-400 mb-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5H7a2 2 0 00-2 2v10a2 2 0 002 2h8a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-3 7h3m-3 4h3m-6-4h.01M9 16h.01"/>
			</svg>'
		/>
	</div>

	<!-- Pagination -->
	{#if pageCount > 1}
		<Pagination
			currentPage={currentPage}
			totalPages={pageCount}
			on:page-change={handlePageChange}
		/>
	{/if}
</div>

<!-- Browse Flows link in empty state -->
{#if !loading && executions.length === 0}
	<div class="flex justify-center mt-4">
		<a 
			href="/view/{data.namespace}/flows"
			class="bg-blue-600 text-white px-4 py-2 rounded-lg text-sm font-medium hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 transition-colors"
		>
			Browse Flows
		</a>
	</div>
{/if}