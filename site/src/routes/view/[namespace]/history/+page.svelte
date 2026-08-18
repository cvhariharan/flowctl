<script lang="ts">
	import { browser } from '$app/environment';
	import { page } from '$app/state';
	import type { PageData } from './$types';
	import PageHeader from '$lib/components/shared/PageHeader.svelte';
	import SearchInput from '$lib/components/shared/SearchInput.svelte';
	import Table from '$lib/components/shared/Table.svelte';
	import Pagination from '$lib/components/shared/Pagination.svelte';
	import StatusBadge from '$lib/components/shared/StatusBadge.svelte';
	import LinkCell from '$lib/components/shared/cells/LinkCell.svelte';
	import MutedTextCell from '$lib/components/shared/cells/MutedTextCell.svelte';
	import UserAvatarCell from '$lib/components/shared/cells/UserAvatarCell.svelte';
	import BadgeCell from '$lib/components/shared/cells/BadgeCell.svelte';
	import { apiClient } from '$lib/apiClient';
	import type { ExecutionSummary, ExecutionsPaginateResponse } from '$lib/types';
	import { DEFAULT_PAGE_SIZE } from '$lib/constants';
	import Header from '$lib/components/shared/Header.svelte';
	import { handleInlineError, showSuccess } from '$lib/utils/errorHandling';
	import { formatDateTime, formatDuration, getStartTime } from '$lib/utils';
	import IconHistory from '@tabler/icons-svelte/icons/history';

	let { data }: { data: PageData } = $props();
	const namespace = $derived(page.params.namespace!);
	const encodedNamespace = $derived(encodeURIComponent(namespace));

	// State
	let executions = $state<ExecutionSummary[]>([]);
	let totalCount = $state(0);
	let pageCount = $state(0);
	let currentPage = $state(data.currentPage);
	let searchQuery = $state(data.searchQuery);
	let loading = $state(true);

	// Handle the async data from load function
	$effect(() => {
		let cancelled = false;

		data.executionsPromise
			.then((result: ExecutionsPaginateResponse) => {
				if (!cancelled) {
					executions = result.executions || [];
					totalCount = result.total_count || 0;
					pageCount = result.page_count || 1;
					loading = false;
				}
			})
			.catch((err: Error) => {
				if (!cancelled) {
					executions = [];
					totalCount = 0;
					pageCount = 0;
					handleInlineError(err, "Unable to Load Execution History");
					loading = false;
				}
			});

		return () => {
			cancelled = true;
		};
	});

	// Table configuration
	let tableColumns = [
		{
			key: 'flow_name',
			header: 'Flow Name',
			sortable: true,
			component: LinkCell,
			componentProps: { href: (row: ExecutionSummary) => `/view/${data.namespace}/flows/${row.flow_id}` }
		},
		{
			key: 'id',
			header: 'Exec ID',
			component: LinkCell,
			componentProps: {
				href: (row: ExecutionSummary) => `/view/${data.namespace}/results/${row.flow_id}/${row.id}`,
				mono: true,
				truncate: 8
			}
		},
		{
			key: 'status',
			header: 'Status',
			sortable: true,
			component: StatusBadge
		},
		{
			key: 'started_at',
			header: 'Started At',
			sortable: true,
			component: MutedTextCell,
			componentProps: { format: (_v: any, row: ExecutionSummary) => formatDateTime(getStartTime(row)) }
		},
		{
			key: 'duration',
			header: 'Duration',
			component: MutedTextCell,
			componentProps: { format: (_v: any, row: ExecutionSummary) => formatDuration(getStartTime(row), row.completed_at) }
		},
		{
			key: 'triggered_by',
			header: 'Triggered By',
			sortable: true,
			component: UserAvatarCell
		},
		{
			key: 'trigger_type',
			header: 'Trigger Type',
			sortable: true,
			component: BadgeCell,
			componentProps: { variant: (row: ExecutionSummary) => row.trigger_type === 'manual' ? 'primary' : 'success' }
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

</script>

<svelte:head>
  <title>Execution History - {page.params.namespace} - Flowctl</title>
</svelte:head>

<Header breadcrumbs={[
  { label: namespace, url: `/view/${encodedNamespace}/flows` },
  { label: "History" }
]}>
  {#snippet children()}
    <SearchInput
      bind:value={searchQuery}
      placeholder="Search executions..."
      {loading}
      onSearch={handleSearch}
    />
  {/snippet}
</Header>

<div class="page-content">
	<!-- Page Header -->
	<PageHeader
		title="Execution History"
		subtitle="View all flow execution history across all flows in this namespace"
	/>

	<!-- Executions Table -->
	<div class="mt-4">
		<Table
			data={executions}
			columns={tableColumns}
			{loading}
			emptyMessage="No execution history found. Executions will appear here once flows are triggered."
			EmptyIconComponent={IconHistory}
			emptyIconSize={64}
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
