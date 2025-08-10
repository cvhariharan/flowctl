<script lang="ts">
	import { browser } from '$app/environment';
	import SearchInput from '$lib/components/shared/SearchInput.svelte';
	import Table from '$lib/components/shared/Table.svelte';
	import Pagination from '$lib/components/shared/Pagination.svelte';
	import GroupModal from './GroupModal.svelte';
	import DeleteModal from '$lib/components/shared/DeleteModal.svelte';
	import { apiClient } from '$lib/apiClient';
	import { handleInlineError, showSuccess } from '$lib/utils/errorHandling';
	import type { Group } from '$lib/types';
	import { DEFAULT_PAGE_SIZE } from '$lib/constants';

	let {
		groups: initialGroups,
		totalCount: initialTotalCount,
		pageCount: initialPageCount
	}: {
		groups: Group[];
		totalCount: number;
		pageCount: number;
	} = $props();

	// State
	let groups = $state(initialGroups);
	let totalCount = $state(initialTotalCount);
	let pageCount = $state(initialPageCount);
	let currentPage = $state(1);
	let searchQuery = $state('');
	let loading = $state(false);
	let showGroupModal = $state(false);
	let showDeleteModal = $state(false);
	let isEditMode = $state(false);
	let editingGroupId = $state<string | null>(null);
	let editingGroupData = $state<Group | null>(null);
	let deleteData = $state<{ id: string; name: string } | null>(null);

	// Table configuration
	let tableColumns = [
		{
			key: 'name',
			header: 'Name',
			render: (_value: any, group: Group) => `
				<div class="flex items-center">
					<div class="w-10 h-10 rounded-lg flex items-center justify-center mr-3 bg-purple-100">
						<i class="ti ti-users-group text-purple-600"></i>
					</div>
					<div>
						<div class="text-sm font-medium text-gray-900">${group.name}</div>
						<div class="text-sm text-gray-500">${group.description || 'No description'}</div>
					</div>
				</div>
			`
		},
		{
			key: 'users',
			header: 'Users',
			render: (_value: any, group: Group) => {
				const userCount = group.users?.length || 0;
				return `<span class="text-gray-900">${userCount} ${userCount === 1 ? 'user' : 'users'}</span>`;
			}
		}
	];

	let tableActions = [
		{
			label: 'Edit',
			onClick: (group: Group) => handleEdit(group.id),
			className: 'text-blue-600 hover:text-blue-800'
		},
		{
			label: 'Delete',
			onClick: (group: Group) => handleDelete(group.id, group.name),
			className: 'text-red-600 hover:text-red-800'
		}
	];

	// Functions
	async function fetchGroups(filter: string = '', pageNumber: number = 1) {
		if (!browser) return;
		
		loading = true;
		try {
			const response = await apiClient.groups.list({
				page: pageNumber,
				count_per_page: DEFAULT_PAGE_SIZE,
				filter: filter || ''
			});

			groups = response.groups || [];
			totalCount = response.total_count || 0;
			pageCount = response.page_count || 1;
		} catch (error) {
			handleInlineError(error, 'Unable to Load Groups List');
		} finally {
			loading = false;
		}
	}

	function handleSearch(query: string) {
		searchQuery = query;
		currentPage = 1;
		fetchGroups(query, 1);
	}

	function handlePageChange(event: CustomEvent<{ page: number }>) {
		currentPage = event.detail.page;
		fetchGroups(searchQuery, currentPage);
	}

	function handleAdd() {
		isEditMode = false;
		editingGroupId = null;
		editingGroupData = null;
		showGroupModal = true;
	}

	async function handleEdit(groupId: string) {
		try {
			loading = true;
			const group = await apiClient.groups.getById(groupId);
			
			isEditMode = true;
			editingGroupId = groupId;
			editingGroupData = group;
			showGroupModal = true;
		} catch (error) {
			handleInlineError(error, 'Unable to Load Group Details');
		} finally {
			loading = false;
		}
	}

	function handleDelete(groupId: string, groupName: string) {
		deleteData = { id: groupId, name: groupName };
		showDeleteModal = true;
	}

	async function handleGroupSave(groupData: any) {
		try {
			if (isEditMode && editingGroupId) {
				await apiClient.groups.update(editingGroupId, groupData);
				showSuccess('Group Updated', `Group "${groupData.name}" has been updated successfully`);
			} else {
				await apiClient.groups.create(groupData);
				showSuccess('Group Created', `Group "${groupData.name}" has been created successfully`);
			}
			showGroupModal = false;
			await fetchGroups(searchQuery, currentPage);
		} catch (error) {
			handleInlineError(error, isEditMode ? 'Unable to Update Group' : 'Unable to Create Group');
			throw error;
		}
	}

	async function handleDeleteConfirm() {
		if (!deleteData) return;
		
		try {
			await apiClient.groups.delete(deleteData.id);
			showSuccess('Group Deleted', `Group "${deleteData.name}" has been deleted successfully`);
			showDeleteModal = false;
			await fetchGroups(searchQuery, currentPage);
		} catch (error) {
			handleInlineError(error, 'Unable to Delete Group');
			throw error;
		}
	}

	function handleModalClose() {
		showGroupModal = false;
		showDeleteModal = false;
		isEditMode = false;
		editingGroupId = null;
		editingGroupData = null;
		deleteData = null;
	}
</script>

<!-- Groups Header Actions -->
<div class="flex items-center justify-between mb-6">
	<!-- Search -->
	<SearchInput
		bind:value={searchQuery}
		placeholder="Search groups..."
		{loading}
		onSearch={handleSearch}
	/>

	<!-- Add Group Button -->
	<button
		onclick={handleAdd}
		class="bg-blue-600 text-white px-4 py-2 rounded-lg text-sm font-medium hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 transition-colors"
	>
		<i class="ti ti-plus text-base mr-2"></i>
		Add Group
	</button>
</div>

<!-- Groups Table -->
<div class="mb-6">
	<Table
		data={groups}
		columns={tableColumns}
		actions={tableActions}
		{loading}
		emptyMessage="No groups found. Get started by adding your first group."
	/>
</div>

<!-- Groups Pagination -->
{#if pageCount > 1}
	<Pagination
		currentPage={currentPage}
		totalPages={pageCount}
		on:page-change={handlePageChange}
	/>
{/if}

<!-- Group Modal -->
{#if showGroupModal}
	<GroupModal
		{isEditMode}
		groupData={editingGroupData}
		onSave={handleGroupSave}
		onClose={handleModalClose}
	/>
{/if}

<!-- Delete Modal -->
{#if showDeleteModal && deleteData}
	<DeleteModal
		title="Delete Group"
		itemName={deleteData.name}
		onConfirm={handleDeleteConfirm}
		onClose={handleModalClose}
	/>
{/if}