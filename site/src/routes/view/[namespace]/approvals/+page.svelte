<script lang="ts">
	import { browser } from '$app/environment';
	import { page } from '$app/state';
	import type { PageData } from './$types';
	import PageHeader from '$lib/components/shared/PageHeader.svelte';
	import SearchInput from '$lib/components/shared/SearchInput.svelte';
	import Table from '$lib/components/shared/Table.svelte';
	import Pagination from '$lib/components/shared/Pagination.svelte';
	import StatusBadge from '$lib/components/shared/StatusBadge.svelte';
	import NameLinkCell from '$lib/components/shared/cells/NameLinkCell.svelte';
	import MutedTextCell from '$lib/components/shared/cells/MutedTextCell.svelte';
	import StatusFilter from '$lib/components/approvals/StatusFilter.svelte';
	import ApprovalDetailsModal from '$lib/components/approvals/ApprovalDetailsModal.svelte';
	import { apiClient } from '$lib/apiClient';
	import type { ApprovalResp, ApprovalsPaginateResponse } from '$lib/types';
	import { DEFAULT_PAGE_SIZE } from '$lib/constants';
	import Header from '$lib/components/shared/Header.svelte';
	import { handleInlineError, showSuccess } from '$lib/utils/errorHandling';
	import { formatDateTime } from '$lib/utils';
	import IconCircleCheck from '@tabler/icons-svelte/icons/circle-check';
	import IconClock from '@tabler/icons-svelte/icons/clock';
	import IconCheck from '@tabler/icons-svelte/icons/check';
	import IconX from '@tabler/icons-svelte/icons/x';

	let { data }: { data: PageData } = $props();
	const namespace = $derived(page.params.namespace!);
	const encodedNamespace = $derived(encodeURIComponent(namespace));

	// State
	let approvals = $state<ApprovalResp[]>([]);
	let totalCount = $state(0);
	let pageCount = $state(0);
	let currentPage = $state(data.currentPage);
	let searchQuery = $state(data.searchQuery);
	let statusFilter = $state(data.statusFilter);
	let loading = $state(true);

	// Handle the async data from load function
	$effect(() => {
		let cancelled = false;

		data.approvalsPromise
			.then((result: ApprovalsPaginateResponse) => {
				if (!cancelled) {
					approvals = result.approvals || [];
					totalCount = result.total_count || 0;
					pageCount = result.page_count || 1;
					loading = false;
				}
			})
			.catch((err: Error) => {
				if (!cancelled) {
					approvals = [];
					totalCount = 0;
					pageCount = 0;
					handleInlineError(err, "Unable to Load Approvals");
					loading = false;
				}
			});

		return () => {
			cancelled = true;
		};
	});

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
			component: NameLinkCell,
			componentProps: {
				icon: IconCircleCheck,
				nameKey: 'action_id',
				subtitleKey: 'id',
				onClick: handleRowClick
			}
		},
		{
			key: 'flow_name',
			header: 'Flow Name',
			sortable: true
		},
		{
			key: 'created_at',
			header: 'Created',
			sortable: true,
			component: MutedTextCell,
			componentProps: { format: (v: any) => formatDateTime(v) }
		},
		{
			key: 'requested_by',
			header: 'Requested By',
			sortable: true
		},
		{
			key: 'exec_id',
			header: 'Execution',
			sortable: true,
			component: MutedTextCell,
			componentProps: { mono: true, truncate: 8 }
		},
		{
			key: 'status',
			header: 'Status',
			sortable: true,
			component: StatusBadge
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



</script>

<svelte:head>
  <title>Approvals - {page.params.namespace} - Flowctl</title>
</svelte:head>

<Header breadcrumbs={[
  { label: namespace, url: `/view/${encodedNamespace}/flows` },
  { label: "Approvals" }
]}>
  {#snippet children()}
    <StatusFilter
      bind:value={statusFilter}
      onChange={handleStatusChange}
    />
    <SearchInput
      bind:value={searchQuery}
      placeholder="Search approval requests..."
      {loading}
      onSearch={handleSearch}
    />
  {/snippet}
</Header>

<div class="page-content">
	<!-- Page Header -->
	<PageHeader
		title="Approvals"
		subtitle="Manage workflow approvals and track their status"
	/>

	<!-- Statistics Cards -->
	<div class="stat-grid mb-4">
		<article class="card p-4">
			<div class="hstack justify-between">
				<div>
					<p class="text-sm text-light">Total Approvals</p>
					<p style="font-size: var(--text-3); font-weight: 700">{totalCount}</p>
				</div>
				<IconCircleCheck size={40} />
			</div>
		</article>
		<article class="card p-4">
			<div class="hstack justify-between">
				<div>
					<p class="text-sm text-light">Pending</p>
					<p style="font-size: var(--text-3); font-weight: 700">{pendingCount}</p>
				</div>
				<IconClock size={40} style="color: var(--warning)" />
			</div>
		</article>
		<article class="card p-4">
			<div class="hstack justify-between">
				<div>
					<p class="text-sm text-light">Approved</p>
					<p style="font-size: var(--text-3); font-weight: 700">{approvedCount}</p>
				</div>
				<IconCheck size={40} style="color: var(--success)" />
			</div>
		</article>
		<article class="card p-4">
			<div class="hstack justify-between">
				<div>
					<p class="text-sm text-light">Rejected</p>
					<p style="font-size: var(--text-3); font-weight: 700">{rejectedCount}</p>
				</div>
				<IconX size={40} style="color: var(--danger)" />
			</div>
		</article>
	</div>

	<!-- Approvals Table -->
	<div class="mt-4">
		<Table
			data={approvals}
			columns={tableColumns}
			{loading}
			emptyMessage="No approvals found. Approvals will appear here when workflows require approval."
			EmptyIconComponent={IconCircleCheck}
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
