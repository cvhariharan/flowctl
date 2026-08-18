<script lang="ts">
	import { browser } from '$app/environment';
	import { page } from '$app/state';
	import type { PageData } from './$types';
	import PageHeader from '$lib/components/shared/PageHeader.svelte';
	import SearchInput from '$lib/components/shared/SearchInput.svelte';
	import Table from '$lib/components/shared/Table.svelte';
	import Pagination from '$lib/components/shared/Pagination.svelte';
	import NodeModal from '$lib/components/nodes/NodeModal.svelte';
	import DeleteModal from '$lib/components/shared/DeleteModal.svelte';
	import { apiClient } from '$lib/apiClient';
	import type { NodeResp, NodeReq, NodeStatsResp, NodesPaginateResponse, CredentialResp, CredentialsPaginateResponse } from '$lib/types';
	import { DEFAULT_PAGE_SIZE } from '$lib/constants';
	import Header from '$lib/components/shared/Header.svelte';
	import { handleInlineError, showSuccess } from '$lib/utils/errorHandling';
	import IconPlus from '@tabler/icons-svelte/icons/plus';
	import IconServer from '@tabler/icons-svelte/icons/server';
	import NameLinkCell from '$lib/components/shared/cells/NameLinkCell.svelte';
	import BadgeCell from '$lib/components/shared/cells/BadgeCell.svelte';
	import TagsCell from '$lib/components/shared/cells/TagsCell.svelte';

	let { data }: { data: PageData } = $props();
	const namespace = $derived(page.params.namespace!);
	const encodedNamespace = $derived(encodeURIComponent(namespace));

	// State
	let nodes = $state<NodeResp[]>([]);
	let totalCount = $state(0);
	let pageCount = $state(0);
	let currentPage = $state(data.currentPage);
	let searchQuery = $state(data.searchQuery);
	let stats = $state<NodeStatsResp>({ total_hosts: 0, qssh_hosts: 0, ssh_hosts: 0 });
	let credentials = $state<CredentialResp[]>([]);
	let loading = $state(true);
	let showModal = $state(false);

	// Handle the async data from load function
	$effect(() => {
		let cancelled = false;

		// Load nodes
		data.nodesPromise
			.then((result: NodesPaginateResponse) => {
				if (!cancelled) {
					nodes = result.nodes || [];
					totalCount = result.total_count || 0;
					pageCount = result.page_count || 1;
					loading = false;
				}
			})
			.catch((err: Error) => {
				if (!cancelled) {
					nodes = [];
					totalCount = 0;
					pageCount = 0;
					handleInlineError(err, "Unable to Load Nodes");
					loading = false;
				}
			});

		// Load stats
		data.statsPromise
			.then((result: NodeStatsResp) => {
				if (!cancelled) {
					stats = result;
				}
			})
			.catch((err: Error) => {
				if (!cancelled) {
					stats = { total_hosts: 0, qssh_hosts: 0, ssh_hosts: 0 };
					handleInlineError(err, "Unable to Load Node Statistics");
				}
			});

		// Load credentials
		data.credentialsPromise
			.then((result: CredentialsPaginateResponse) => {
				if (!cancelled) {
					credentials = result.credentials || [];
				}
			})
			.catch((err: Error) => {
				if (!cancelled) {
					credentials = [];
					handleInlineError(err, "Unable to Load Credentials");
				}
			});

		return () => {
			cancelled = true;
		};
	});
	let isEditMode = $state(false);
	let editingNodeId = $state<string | null>(null);
	let editingNodeData = $state<NodeResp | null>(null);
	let showDeleteModal = $state(false);
	let deleteNodeId = $state<string | null>(null);
	let deleteNodeName = $state('');


	// Table configuration
	let tableColumns = [
		{
			key: 'name',
			header: 'Node',
			sortable: true,
			component: NameLinkCell,
			componentProps: {
				icon: IconServer,
				subtitleKey: 'id',
				onClick: (row: NodeResp) => handleEdit(row.id)
			}
		},
		{ key: 'hostname', header: 'Hostname', sortable: true },
		{ key: 'port', header: 'Port', sortable: true },
		{ key: 'username', header: 'Username', sortable: true },
		{ key: 'os_family', header: 'OS Family', sortable: true },
		{
			key: 'connection_type',
			header: 'Connection Type',
			sortable: true,
			component: BadgeCell,
			componentProps: {
				variant: (row: NodeResp) => row.connection_type === 'qssh' ? 'success' : 'primary',
				label: (row: NodeResp) => row.connection_type?.toUpperCase() || 'N/A'
			}
		},
		{
			key: 'tags',
			header: 'Tags',
			component: TagsCell
		}
	];

	let tableActions = [
		{
			label: 'Edit',
			onClick: (node: NodeResp) => handleEdit(node.id),
			className: 'text-link border-link hover:bg-link-hover'
		},
		{
			label: 'Delete',
			onClick: (node: NodeResp) => handleDelete(node.id),
			className: 'text-danger-600 border-danger-600 hover:bg-danger-100'
		}
	];

	async function fetchStats() {
		if (!browser) return;

		try {
			stats = await apiClient.nodes.getStats(data.namespace);
		} catch (error) {
			handleInlineError(error, 'Unable to Load Node Statistics');
		}
	}

	async function fetchNodes(filter: string = '', pageNumber: number = 1) {
		if (!browser) return;

		loading = true;
		try {
			const response = await apiClient.nodes.list(data.namespace, {
				page: pageNumber,
				count_per_page: DEFAULT_PAGE_SIZE,
				filter: filter
			});

			nodes = response.nodes || [];
			totalCount = response.total_count || 0;
			pageCount = response.page_count || 1;
		} catch (error) {
			handleInlineError(error, 'Unable to Load Nodes List');
		} finally {
			loading = false;
		}
	}

	function handleSearch(query: string) {
		searchQuery = query;
		fetchNodes(query);
	}

	function handlePageChange(event: CustomEvent<{ page: number }>) {
		currentPage = event.detail.page;
		fetchNodes('', currentPage);
	}

	function handleAdd() {
		isEditMode = false;
		editingNodeId = null;
		editingNodeData = null;
		showModal = true;
	}

	async function handleEdit(nodeId: string) {
		try {
			const node = await apiClient.nodes.getById(data.namespace, nodeId);

			isEditMode = true;
			editingNodeId = nodeId;
			editingNodeData = node;
			showModal = true;
		} catch (error) {
			handleInlineError(error, 'Unable to Load Node Details');
		}
	}

	function handleDelete(nodeId: string) {
		const node = nodes.find(n => n.id === nodeId);
		if (node) {
			deleteNodeId = nodeId;
			deleteNodeName = node.name;
			showDeleteModal = true;
		}
	}

	async function confirmDelete() {
		if (!deleteNodeId) return;

		try {
			await apiClient.nodes.delete(data.namespace, deleteNodeId);
			closeDeleteModal(); // Close modal after successful deletion
			showSuccess('Node deleted', `Node ${deleteNodeName} has been successfully deleted.`);
			await Promise.all([fetchNodes(), fetchStats()]);
		} catch (error) {
			handleInlineError(error, 'Unable to Delete Node');
		}
	}

	function closeDeleteModal() {
		showDeleteModal = false;
		deleteNodeId = null;
		deleteNodeName = '';
	}

	async function handleNodeSave(nodeData: NodeReq) {
		try {
			if (isEditMode && editingNodeId) {
				await apiClient.nodes.update(data.namespace, editingNodeId, nodeData);
				showSuccess('Node updated', `Node ${nodeData.name} has been successfully updated.`);
			} else {
				await apiClient.nodes.create(data.namespace, nodeData);
				showSuccess('Node created', `Node ${nodeData.name} has been successfully created.`);
			}
			showModal = false;
			await Promise.all([fetchNodes(), fetchStats()]);
		} catch (error) {
			handleInlineError(error, 'Unable to Save Node');
		}
	}

	function handleModalClose() {
		showModal = false;
		isEditMode = false;
		editingNodeId = null;
		editingNodeData = null;
	}

</script>

<svelte:head>
  <title>Nodes - {page.params.namespace} - Flowctl</title>
</svelte:head>

<Header breadcrumbs={[
  { label: namespace, url: `/view/${encodedNamespace}/flows` },
  { label: "Nodes" }
]}>
  {#snippet children()}
    <SearchInput
      bind:value={searchQuery}
      placeholder="Search Nodes..."
      {loading}
      onSearch={handleSearch}
    />
  {/snippet}
</Header>

<div class="page-content">
	<!-- Page Header -->
	<PageHeader
		title="Nodes"
		subtitle="Manage remote nodes that run flows"
		actions={[
			{
				label: 'Add',
				onClick: handleAdd,
				variant: 'primary',
				IconComponent: IconPlus,
				iconSize: 16
			}
		]}
	/>

	<!-- Statistics Cards -->
	<div class="stat-grid">
		<article class="card p-4">
			<div class="hstack justify-between">
				<div>
					<p class="text-sm text-light">Total Hosts</p>
					<p style="font-size: var(--text-3); font-weight: 700">{stats.total_hosts}</p>
				</div>
				<IconServer size={24} />
			</div>
		</article>
		<article class="card p-4">
			<div class="hstack justify-between">
				<div>
					<p class="text-sm text-light">QSSH Hosts</p>
					<p style="font-size: var(--text-3); font-weight: 700">{stats.qssh_hosts}</p>
				</div>
				<IconServer size={24} style="color: var(--success)" />
			</div>
		</article>
		<article class="card p-4">
			<div class="hstack justify-between">
				<div>
					<p class="text-sm text-light">SSH Hosts</p>
					<p style="font-size: var(--text-3); font-weight: 700">{stats.ssh_hosts}</p>
				</div>
				<IconServer size={24} />
			</div>
		</article>
	</div>

	<!-- Nodes Table -->
	<div class="mt-4">
		<Table
			data={nodes}
			columns={tableColumns}
			actions={tableActions}
			{loading}
			emptyMessage="No nodes found. Get started by adding your first node."
			EmptyIconComponent={IconServer}
			emptyIconSize={64}
			emptyActionLabel="Add your first node"
			onEmptyAction={handleAdd}
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

<!-- Node Modal -->
{#if showModal}
	<NodeModal
		{isEditMode}
		nodeData={editingNodeData}
		{credentials}
		onSave={handleNodeSave}
		onClose={handleModalClose}
	/>
{/if}

<!-- Delete Modal -->
{#if showDeleteModal}
	<DeleteModal
		title="Delete Node"
		itemName={deleteNodeName}
		onConfirm={confirmDelete}
		onClose={closeDeleteModal}
	/>
{/if}
