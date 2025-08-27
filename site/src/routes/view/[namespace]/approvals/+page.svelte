<script lang="ts">
	import { browser } from '$app/environment';
	import { page } from '$app/state';
	import type { PageData } from './$types';
	import PageHeader from '$lib/components/shared/PageHeader.svelte';
	import SearchInput from '$lib/components/shared/SearchInput.svelte';
	import Table from '$lib/components/shared/Table.svelte';
	import Pagination from '$lib/components/shared/Pagination.svelte';
	import StatCard from '$lib/components/shared/StatCard.svelte';
	import StatusBadge from '$lib/components/shared/StatusBadge.svelte';
	import ApprovalIdCell from '$lib/components/approvals/ApprovalIdCell.svelte';
	import StatusFilter from '$lib/components/approvals/StatusFilter.svelte';
	import ApprovalDetailsModal from '$lib/components/approvals/ApprovalDetailsModal.svelte';
	import { apiClient } from '$lib/apiClient';
	import type { ApprovalResp } from '$lib/types';
	import { DEFAULT_PAGE_SIZE } from '$lib/constants';
	import Header from '$lib/components/shared/Header.svelte';
	import { handleInlineError, showSuccess } from '$lib/utils/errorHandling';

	let { data }: { data: PageData } = $props();

	// State
	let approvals = $state(data.approvals);
	let totalCount = $state(data.totalCount);
	let pageCount = $state(data.pageCount);
	let currentPage = $state(data.currentPage);
	let searchQuery = $state(data.searchQuery);
	let statusFilter = $state(data.statusFilter);
	let loading = $state(false);

	// Modal state
	let showModal = $state(false);
	let selectedApprovalId = $state<string | null>(null);

	// Computed statistics
	let pendingCount = $derived(approvals.filter(approval => approval.status === 'pending').length);
	let approvedCount = $derived(approvals.filter(approval => approval.status === 'approved').length);
	let rejectedCount = $derived(approvals.filter(approval => approval.status === 'rejected').length);

	// Table configuration
	let tableColumns = [
		{
			key: 'id',
			header: 'Approval',
			component: ApprovalIdCell
		},
		{
			key: 'status',
			header: 'Status',
			sortable: true,
			component: StatusBadge
		},
		{
			key: 'exec_id',
			header: 'Execution',
			sortable: true,
			render: (_value: any, approval: ApprovalResp) => `
				<span class="font-mono text-sm text-gray-900">${approval.exec_id.substring(0, 8)}</span>
			`
		},
		{
			key: 'requested_by',
			header: 'Requested By',
			sortable: true
		},
		{
			key: 'created_at',
			header: 'Created',
			sortable: true,
			render: (_value: any, approval: ApprovalResp) => formatDate(approval.created_at)
		}
	];



	function handleRowClick(row: ApprovalResp) {
		selectedApprovalId = row.id;
		showModal = true;
	}

	async function fetchApprovals(filter: string = '', status: string = '', pageNumber: number = 1) {
		if (!browser) return;
		
		loading = true;
		try {
			const response = await apiClient.approvals.list(data.namespace, {
				page: pageNumber,
				count_per_page: DEFAULT_PAGE_SIZE,
				filter: filter || undefined,
				status: status as any || undefined
			});

			approvals = response.approvals || [];
			totalCount = response.total_count || 0;
			pageCount = response.page_count || 1;
		} catch (error) {
			handleInlineError(error, 'Unable to Load Approvals List');
		} finally {
			loading = false;
		}
	}

	function handleSearch(query: string) {
		searchQuery = query;
		currentPage = 1;
		fetchApprovals(query, statusFilter, 1);
	}

	function handleStatusChange(status: string) {
		statusFilter = status;
		currentPage = 1;
		fetchApprovals(searchQuery, status, 1);
	}

	function handlePageChange(event: CustomEvent<{ page: number }>) {
		currentPage = event.detail.page;
		fetchApprovals(searchQuery, statusFilter, currentPage);
	}

	async function handleApprove(approvalId: string) {
		try {
			await apiClient.approvals.action(data.namespace, approvalId, { action: 'approve' });
			await fetchApprovals(searchQuery, statusFilter, currentPage);
			showSuccess('Approval Approved', 'The approval has been approved successfully');
		} catch (error) {
			handleInlineError(error, 'Unable to Approve Request');
		}
	}

	async function handleReject(approvalId: string) {
		try {
			await apiClient.approvals.action(data.namespace, approvalId, { action: 'reject' });
			await fetchApprovals(searchQuery, statusFilter, currentPage);
			showSuccess('Approval Rejected', 'The approval has been rejected successfully');
		} catch (error) {
			handleInlineError(error, 'Unable to Reject Request');
		}
	}

	function formatDate(dateString: string): string {
		if (!dateString) return 'Never';
		const date = new Date(dateString);
		const now = new Date();
		const diffMs = now.getTime() - date.getTime();
		const diffHours = Math.floor(diffMs / (1000 * 60 * 60));
		const diffDays = Math.floor(diffHours / 24);

		if (diffHours < 1) return 'Less than 1 hour ago';
		if (diffHours < 24) return `${diffHours} hours ago`;
		if (diffDays < 7) return `${diffDays} days ago`;
		return date.toLocaleDateString();
	}


</script>

<svelte:head>
  <title>Approvals - {page.params.namespace} - Flowctl</title>
</svelte:head>

<Header breadcrumbs={[
  { label: page.params.namespace!, url: `/view/${page.params.namespace}/flows` },
  { label: "Approvals" }
]}>
  {#snippet children()}
    <StatusFilter
      bind:value={statusFilter}
      onChange={handleStatusChange}
    />
    <SearchInput
      bind:value={searchQuery}
      placeholder="Search by Action ID or Exec ID..."
      {loading}
      onSearch={handleSearch}
    />
  {/snippet}
</Header>

<div class="p-12">
	<!-- Page Header -->
	<PageHeader 
		title="Approvals"
		subtitle="Manage workflow approvals and track their status"
	/>

	<!-- Statistics Cards -->
	<div class="grid grid-cols-1 md:grid-cols-4 gap-6 mb-6">
		<StatCard
			title="Total Approvals"
			value={totalCount}
			icon='<svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>'
			color="blue"
		/>
		<StatCard
			title="Pending"
			value={pendingCount}
			icon='<svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>'
			color="yellow"
		/>
		<StatCard
			title="Approved"
			value={approvedCount}
			icon='<svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/></svg>'
			color="green"
		/>
		<StatCard
			title="Rejected"
			value={rejectedCount}
			icon='<svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/></svg>'
			color="red"
		/>
	</div>

	<!-- Approvals Table -->
	<div class="pt-6">
		<Table
			data={approvals}
			columns={tableColumns}
			onRowClick={handleRowClick}
			{loading}
			emptyMessage="No approvals found. Approvals will appear here when workflows require approval."
			emptyIcon='<svg class="w-16 h-16 text-gray-400 mb-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"/>
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

<!-- Approval Details Modal -->
{#if selectedApprovalId}
	<ApprovalDetailsModal
		bind:open={showModal}
		approvalId={selectedApprovalId}
		namespace={data.namespace}
		onApprove={handleApprove}
		onReject={handleReject}
	/>
{/if}