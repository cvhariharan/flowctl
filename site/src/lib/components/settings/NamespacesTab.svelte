<script lang="ts">
	import { browser } from '$app/environment';
	import SearchInput from '$lib/components/shared/SearchInput.svelte';
	import Table from '$lib/components/shared/Table.svelte';
	import Pagination from '$lib/components/shared/Pagination.svelte';
	import NamespaceModal from './NamespaceModal.svelte';
	import DeleteModal from '$lib/components/shared/DeleteModal.svelte';
	import { apiClient } from '$lib/apiClient';
	import { handleInlineError, showSuccess } from '$lib/utils/errorHandling';
	import type { NamespaceResp } from '$lib/types';
	import { DEFAULT_PAGE_SIZE } from '$lib/constants';
	import { IconPlus } from '@tabler/icons-svelte';

	let {
		namespaces: initialNamespaces,
		totalCount: initialTotalCount,
		pageCount: initialPageCount
	}: {
		namespaces: NamespaceResp[];
		totalCount: number;
		pageCount: number;
	} = $props();

	// State
	let namespaces = $state(initialNamespaces);
	let totalCount = $state(initialTotalCount);
	let pageCount = $state(initialPageCount);
	let currentPage = $state(1);
	let searchQuery = $state('');
	let loading = $state(false);
	let showNamespaceModal = $state(false);
	let showDeleteModal = $state(false);
	let isEditMode = $state(false);
	let editingNamespaceId = $state<string | null>(null);
	let editingNamespaceData = $state<NamespaceResp | null>(null);
	let deleteData = $state<{ id: string; name: string } | null>(null);

	// Table configuration
	let tableColumns = [
		{
			key: 'name',
			header: 'Name',
			render: (_value: any, namespace: NamespaceResp) => {
				const firstLetter = namespace.name.charAt(0).toUpperCase();
				const colors = ['bg-danger-100 text-danger-600', 'bg-primary-100 text-primary-600', 'bg-success-100 text-success-600', 'bg-warning-100 text-warning-600', 'bg-primary-100 text-primary-600', 'bg-pink-100 text-pink-600', 'bg-indigo-100 text-indigo-600'];
				const colorIndex = namespace.name.charCodeAt(0) % colors.length;
				const colorClass = colors[colorIndex];
				
				return `
					<div class="flex items-center">
						<div class="w-10 h-10 rounded-lg flex items-center justify-center mr-3 ${colorClass} font-medium text-sm">
							${firstLetter}
						</div>
						<div>
							<div class="text-sm font-medium text-gray-900">${namespace.name}</div>
							<div class="text-sm text-gray-500">ID: ${namespace.id}</div>
						</div>
					</div>
				`;
			}
		}
	];

	let tableActions = [
		{
			label: 'Edit',
			onClick: (namespace: NamespaceResp) => handleEdit(namespace.id),
			className: 'text-primary-600 hover:text-primary-800'
		},
		{
			label: 'Delete',
			onClick: (namespace: NamespaceResp) => handleDelete(namespace.id, namespace.name),
			className: 'text-danger-600 hover:text-danger-800'
		}
	];

	// Functions
	async function fetchNamespaces(filter: string = '', pageNumber: number = 1) {
		if (!browser) return;
		
		loading = true;
		try {
			const response = await apiClient.namespaces.list({
				page: pageNumber,
				count_per_page: DEFAULT_PAGE_SIZE,
				filter: filter || ''
			});

			namespaces = response.namespaces || [];
			totalCount = response.total_count || 0;
			pageCount = response.page_count || 1;
		} catch (error) {
			handleInlineError(error, 'Unable to Load Namespaces List');
		} finally {
			loading = false;
		}
	}

	function handleSearch(query: string) {
		searchQuery = query;
		currentPage = 1;
		fetchNamespaces(query, 1);
	}

	function handlePageChange(event: CustomEvent<{ page: number }>) {
		currentPage = event.detail.page;
		fetchNamespaces(searchQuery, currentPage);
	}

	function handleAdd() {
		isEditMode = false;
		editingNamespaceId = null;
		editingNamespaceData = null;
		showNamespaceModal = true;
	}

	async function handleEdit(namespaceId: string) {
		try {
			loading = true;
			const namespace = await apiClient.namespaces.getById(namespaceId);
			
			isEditMode = true;
			editingNamespaceId = namespaceId;
			editingNamespaceData = namespace;
			showNamespaceModal = true;
		} catch (error) {
			handleInlineError(error, 'Unable to Load Namespace Details');
		} finally {
			loading = false;
		}
	}

	function handleDelete(namespaceId: string, namespaceName: string) {
		deleteData = { id: namespaceId, name: namespaceName };
		showDeleteModal = true;
	}

	async function handleNamespaceSave(namespaceData: any) {
		try {
			if (isEditMode && editingNamespaceId) {
				await apiClient.namespaces.update(editingNamespaceId, namespaceData);
				showSuccess('Namespace Updated', `Namespace "${namespaceData.name}" has been updated successfully`);
			} else {
				await apiClient.namespaces.create(namespaceData);
				showSuccess('Namespace Created', `Namespace "${namespaceData.name}" has been created successfully`);
			}
			showNamespaceModal = false;
			await fetchNamespaces(searchQuery, currentPage);
		} catch (error) {
			handleInlineError(error, isEditMode ? 'Unable to Update Namespace' : 'Unable to Create Namespace');
		}
	}

	async function handleDeleteConfirm() {
		if (!deleteData) return;

		try {
			await apiClient.namespaces.delete(deleteData.id);
			showSuccess('Namespace Deleted', `Namespace "${deleteData.name}" has been deleted successfully`);
			showDeleteModal = false;
			await fetchNamespaces(searchQuery, currentPage);
		} catch (error) {
			handleInlineError(error, 'Unable to Delete Namespace');
		}
	}

	function handleModalClose() {
		showNamespaceModal = false;
		showDeleteModal = false;
		isEditMode = false;
		editingNamespaceId = null;
		editingNamespaceData = null;
		deleteData = null;
	}
</script>

<!-- Namespaces Header Actions -->
<div class="flex items-center justify-between mb-6">
	<!-- Search -->
	<SearchInput
		bind:value={searchQuery}
		placeholder="Search namespaces..."
		{loading}
		onSearch={handleSearch}
	/>

	<!-- Add Namespace Button -->
	<button
		onclick={handleAdd}
		class="bg-primary-500 text-white px-4 py-2 rounded-lg text-sm font-medium hover:bg-primary-600 focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2 transition-colors flex items-center cursor-pointer"
	>
		<IconPlus class="mr-2" size={16} />
		Add Namespace
	</button>
</div>

<!-- Namespaces Table -->
<div class="mb-6">
	<Table
		data={namespaces}
		columns={tableColumns}
		actions={tableActions}
		{loading}
		emptyMessage="No namespaces found. Get started by adding your first namespace."
	/>
</div>

<!-- Namespaces Pagination -->
{#if pageCount > 1}
	<Pagination
		currentPage={currentPage}
		totalPages={pageCount}
		on:page-change={handlePageChange}
	/>
{/if}

<!-- Namespace Modal -->
{#if showNamespaceModal}
	<NamespaceModal
		{isEditMode}
		namespaceData={editingNamespaceData}
		onSave={handleNamespaceSave}
		onClose={handleModalClose}
	/>
{/if}

<!-- Delete Modal -->
{#if showDeleteModal && deleteData}
	<DeleteModal
		title="Delete Namespace"
		itemName={deleteData.name}
		onConfirm={handleDeleteConfirm}
		onClose={handleModalClose}
	/>
{/if}