<script lang="ts">
	import { browser } from '$app/environment';
	import { page } from '$app/state';
	import type { PageData } from './$types';
	import PageHeader from '$lib/components/shared/PageHeader.svelte';
	import SearchInput from '$lib/components/shared/SearchInput.svelte';
	import Table from '$lib/components/shared/Table.svelte';
	import Pagination from '$lib/components/shared/Pagination.svelte';
	import StatCard from '$lib/components/shared/StatCard.svelte';
	import NodeModal from '$lib/components/nodes/NodeModal.svelte';
	import { apiClient } from '$lib/apiClient';
	import type { NodeResp, NodeReq } from '$lib/types';
    import { DEFAULT_PAGE_SIZE } from '$lib/constants';
    import Header from '$lib/components/shared/Header.svelte';

	let { data }: { data: PageData } = $props();

	// State
	let nodes = $state(data.nodes);
	let totalCount = $state(data.totalCount);
	let pageCount = $state(data.pageCount);
	let currentPage = $state(data.currentPage);
	let searchQuery = $state(data.searchQuery);
	let loading = $state(false);
	let showModal = $state(false);
	let isEditMode = $state(false);
	let editingNodeId = $state<string | null>(null);
	let editingNodeData = $state<NodeResp | null>(null);

	// Computed values
	let linuxCount = $derived(nodes.filter(node => node.os_family === 'linux').length);
	let windowsCount = $derived(nodes.filter(node => node.os_family === 'windows').length);


	// Table configuration
	let tableColumns = [
		{
			key: 'name',
			header: 'Node',
			render: (_value: any, node: NodeResp) => `
				<div class="flex items-center">
					<div class="w-10 h-10 bg-blue-100 rounded-lg flex items-center justify-center mr-3">
						<i class="ti ti-server text-blue-600"></i>
					</div>
					<div>
						<div class="text-sm font-medium text-gray-900">${node.name}</div>
						<div class="text-sm text-gray-500">${node.id}</div>
					</div>
				</div>
			`
		},
		{ key: 'hostname', header: 'Hostname' },
		{ key: 'port', header: 'Port' },
		{ key: 'username', header: 'Username' },
		{
			key: 'os_family',
			header: 'OS Family',
			render: (_value: any, node: NodeResp) => `
				<span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
					node.os_family === 'linux' 
						? 'bg-green-100 text-green-800' 
						: 'bg-blue-100 text-blue-800'
				}">${node.os_family}</span>
			`
		},
		{
			key: 'tags',
			header: 'Tags',
			render: (_value: any, node: NodeResp) => node.tags && node.tags.length > 0 
				? `<div class="flex flex-wrap gap-1">
					${node.tags.map(tag => 
						`<span class="inline-flex items-center px-2 py-1 rounded-full text-xs font-medium bg-gray-100 text-gray-800">${tag}</span>`
					).join('')}
				</div>`
				: '<span class="text-xs text-gray-400">No tags</span>'
		}
	];

	let tableActions = [
		{
			label: 'Edit',
			onClick: (node: NodeResp) => handleEdit(node.id),
			className: 'text-blue-600 hover:text-blue-800'
		},
		{
			label: 'Delete',
			onClick: (node: NodeResp) => handleDelete(node.id),
			className: 'text-red-600 hover:text-red-800'
		}
	];

	// Functions
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
			console.error('Failed to fetch nodes:', error);
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
			console.error('Failed to load node:', error);
		}
	}

	async function handleDelete(nodeId: string) {
		if (!confirm('Are you sure you want to delete this node?')) return;

		try {
			await apiClient.nodes.delete(data.namespace, nodeId);
			await fetchNodes();
		} catch (error) {
			console.error('Failed to delete node:', error);
		}
	}

	async function handleNodeSave(nodeData: NodeReq) {
		try {
			if (isEditMode && editingNodeId) {
				await apiClient.nodes.update(data.namespace, editingNodeId, nodeData);
			} else {
				await apiClient.nodes.create(data.namespace, nodeData);
			}
			showModal = false;
			await fetchNodes();
		} catch (error) {
			console.error('Failed to save node:', error);
			throw error;
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

<Header breadcrumbs={[`${page.params.namespace}`, "Nodes"]}>
  {#snippet children()}
    <SearchInput
      bind:value={searchQuery}
      placeholder="Search Nodes..."
      {loading}
      onSearch={handleSearch}
    />
  {/snippet}
</Header>

<div class="p-12">
	<!-- Page Header -->
	<PageHeader 
		title="Nodes"
		subtitle="Manage remote nodes that run flows"
		actions={[
			{
				label: 'Add Node',
				onClick: handleAdd,
				variant: 'primary',
				icon: '<i class="ti ti-plus"></i>'
			}
		]}
	/>

	<!-- Statistics Cards -->
	<div class="grid grid-cols-1 md:grid-cols-3 gap-6">
		<StatCard
			title="Total Nodes"
			value={totalCount}
			icon='<i class="ti ti-server w-6 h-6"></i>'
			color="blue"
		/>
		<StatCard
			title="Linux Nodes"
			value={linuxCount}
			icon='<i class="ti ti-server w-6 h-6"></i>'
			color="green"
		/>
		<StatCard
			title="Windows Nodes"
			value={windowsCount}
			icon='<i class="ti ti-server w-6 h-6"></i>'
			color="purple"
		/>
	</div>

	<!-- Nodes Table -->
	<div class="pt-6">
		<Table
			data={nodes}
			columns={tableColumns}
			actions={tableActions}
			{loading}
			emptyMessage="No nodes found. Get started by adding your first node."
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
		credentials={data.credentials}
		onSave={handleNodeSave}
		onClose={handleModalClose}
	/>
{/if}