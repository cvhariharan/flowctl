<script lang="ts">
	import { browser } from '$app/environment';
	import SearchInput from '$lib/components/shared/SearchInput.svelte';
	import Table from '$lib/components/shared/Table.svelte';
	import Pagination from '$lib/components/shared/Pagination.svelte';
	import GroupModal from './GroupModal.svelte';
	import DeleteModal from '$lib/components/shared/DeleteModal.svelte';
	import { apiClient } from '$lib/apiClient';
	import { handleInlineError, showSuccess } from '$lib/utils/errorHandling';
	import type { Group, GroupWithUsers } from '$lib/types';
	import { DEFAULT_PAGE_SIZE } from '$lib/constants';
	import IconPlus from '@tabler/icons-svelte/icons/plus';

	let {
		groups: initialGroups,
		totalCount: initialTotalCount,
		pageCount: initialPageCount,
		refreshTrigger
	}: {
		groups: Group[];
		totalCount: number;
		pageCount: number;
		refreshTrigger: boolean;
	} = $props();

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
	let editingGroupData = $state<GroupWithUsers | null>(null);
	let deleteData = $state<{ id: string; name: string } | null>(null);

	const avatarColors = [
		{ bg: 'color-mix(in srgb, var(--danger) 15%, transparent)', fg: 'var(--danger)' },
		{ bg: 'color-mix(in srgb, var(--primary) 15%, transparent)', fg: 'var(--primary)' },
		{ bg: 'color-mix(in srgb, var(--success) 15%, transparent)', fg: 'var(--success)' },
		{ bg: 'color-mix(in srgb, var(--warning) 15%, transparent)', fg: 'var(--warning)' },
		{ bg: 'color-mix(in srgb, var(--primary) 15%, transparent)', fg: 'var(--primary)' },
	];

	let tableColumns = [
		{
			key: 'name',
			header: 'Name',
			render: (_value: any, group: Group) => {
				const firstLetter = group.name.charAt(0).toUpperCase();
				const c = avatarColors[group.name.charCodeAt(0) % avatarColors.length];

				return `
					<div class="name-cell">
						<div class="avatar" style="background:${c.bg};color:${c.fg}">
							${firstLetter}
						</div>
						<div>
							<div class="name-link" onclick="document.dispatchEvent(new CustomEvent('editGroup', {detail: {id: '${group.id}'}}))">${group.name}</div>
							<div class="name-sub">${group.description || 'No description'}</div>
						</div>
					</div>
				`;
			}
		},
		{
			key: 'users',
			header: 'Users',
			render: (_value: any, group: Group) => {
				const userCount = group.users?.length || 0;
				return `${userCount} ${userCount === 1 ? 'user' : 'users'}`;
			}
		}
	];

	let tableActions = [
		{
			label: 'Edit',
			onClick: (group: Group) => handleEdit(group.id),
		},
		{
			label: 'Delete',
			onClick: (group: Group) => handleDelete(group.id, group.name),
		}
	];

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

	if (browser) {
		document.addEventListener('editGroup', ((event: CustomEvent) => {
			handleEdit(event.detail.id);
		}) as EventListener);
	}

	$effect(() => {
		refreshTrigger;
		fetchGroups('', 1);
	});
</script>

<!-- Groups Header Actions -->
<div class="hstack mb-4 justify-between">
	<SearchInput
		bind:value={searchQuery}
		placeholder="Search groups..."
		{loading}
		onSearch={handleSearch}
	/>

	<button onclick={handleAdd}>
		<IconPlus size={16} />
		Add Group
	</button>
</div>

<!-- Groups Table -->
<div class="mb-4">
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

<style>
	.mb-4 { margin-bottom: 1.5rem; }
</style>
