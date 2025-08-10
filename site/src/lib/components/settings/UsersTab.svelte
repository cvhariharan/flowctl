<script lang="ts">
	import { browser } from '$app/environment';
	import SearchInput from '$lib/components/shared/SearchInput.svelte';
	import Table from '$lib/components/shared/Table.svelte';
	import Pagination from '$lib/components/shared/Pagination.svelte';
	import UserModal from './UserModal.svelte';
	import DeleteModal from '$lib/components/shared/DeleteModal.svelte';
	import { apiClient } from '$lib/apiClient';
	import { handleInlineError, showSuccess } from '$lib/utils/errorHandling';
	import type { User, Group, UserWithGroups } from '$lib/types';
	import { DEFAULT_PAGE_SIZE } from '$lib/constants';

	let {
		users: initialUsers,
		totalCount: initialTotalCount,
		pageCount: initialPageCount,
		groups
	}: {
		users: User[];
		totalCount: number;
		pageCount: number;
		groups: Group[];
	} = $props();

	// State
	let users = $state(initialUsers);
	let totalCount = $state(initialTotalCount);
	let pageCount = $state(initialPageCount);
	let currentPage = $state(1);
	let searchQuery = $state('');
	let loading = $state(false);
	let showUserModal = $state(false);
	let showDeleteModal = $state(false);
	let isEditMode = $state(false);
	let editingUserId = $state<string | null>(null);
	let editingUserData = $state<UserWithGroups | null>(null);
	let deleteData = $state<{ id: string; name: string } | null>(null);

	// Table configuration
	let tableColumns = [
		{
			key: 'name',
			header: 'Name',
			render: (_value: any, user: User) => `
				<div class="flex items-center">
					<div class="w-10 h-10 rounded-lg flex items-center justify-center mr-3 bg-blue-100">
						<i class="ti ti-user text-blue-600"></i>
					</div>
					<div>
						<div class="text-sm font-medium text-gray-900">${user.name}</div>
						<div class="text-sm text-gray-500">${user.username}</div>
					</div>
				</div>
			`
		},
		{
			key: 'groups',
			header: 'Groups',
			render: (_value: any, user: User) => {
				const userGroups = user.groups || [];
				if (userGroups.length === 0) {
					return '<span class="text-gray-400 text-sm">No groups</span>';
				}
				
				let html = '<div class="flex flex-wrap gap-1 items-center">';
				
				// Show first 3 groups
				userGroups.slice(0, 3).forEach(group => {
					html += `<span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-blue-100 text-blue-800">${group.name}</span>`;
				});
				
				// Show +N more if there are more than 3
				if (userGroups.length > 3) {
					html += `<span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-gray-100 text-gray-800">+${userGroups.length - 3}</span>`;
				}
				
				html += '</div>';
				return html;
			}
		}
	];

	let tableActions = [
		{
			label: 'Edit',
			onClick: (user: User) => handleEdit(user.id),
			className: 'text-blue-600 hover:text-blue-800'
		},
		{
			label: 'Delete',
			onClick: (user: User) => handleDelete(user.id, user.name),
			className: 'text-red-600 hover:text-red-800'
		}
	];

	// Functions
	async function fetchUsers(filter: string = '', pageNumber: number = 1) {
		if (!browser) return;
		
		loading = true;
		try {
			const response = await apiClient.users.list({
				page: pageNumber,
				count_per_page: DEFAULT_PAGE_SIZE,
				filter: filter || ''
			});

			users = response.users || [];
			totalCount = response.total_count || 0;
			pageCount = response.page_count || 1;
		} catch (error) {
			handleInlineError(error, 'Unable to Load Users List');
		} finally {
			loading = false;
		}
	}

	function handleSearch(query: string) {
		searchQuery = query;
		currentPage = 1;
		fetchUsers(query, 1);
	}

	function handlePageChange(event: CustomEvent<{ page: number }>) {
		currentPage = event.detail.page;
		fetchUsers(searchQuery, currentPage);
	}

	function handleAdd() {
		isEditMode = false;
		editingUserId = null;
		editingUserData = null;
		showUserModal = true;
	}

	async function handleEdit(userId: string) {
		try {
			loading = true;
			const user = await apiClient.users.getById(userId);
			
			isEditMode = true;
			editingUserId = userId;
			editingUserData = user;
			showUserModal = true;
		} catch (error) {
			handleInlineError(error, 'Unable to Load User Details');
		} finally {
			loading = false;
		}
	}

	function handleDelete(userId: string, userName: string) {
		deleteData = { id: userId, name: userName };
		showDeleteModal = true;
	}

	async function handleUserSave(userData: any) {
		try {
			if (isEditMode && editingUserId) {
				await apiClient.users.update(editingUserId, userData);
				showSuccess('User Updated', `User "${userData.name}" has been updated successfully`);
			} else {
				await apiClient.users.create(userData);
				showSuccess('User Created', `User "${userData.name}" has been created successfully`);
			}
			showUserModal = false;
			await fetchUsers(searchQuery, currentPage);
		} catch (error) {
			handleInlineError(error, isEditMode ? 'Unable to Update User' : 'Unable to Create User');
			throw error;
		}
	}

	async function handleDeleteConfirm() {
		if (!deleteData) return;
		
		try {
			await apiClient.users.delete(deleteData.id);
			showSuccess('User Deleted', `User "${deleteData.name}" has been deleted successfully`);
			showDeleteModal = false;
			await fetchUsers(searchQuery, currentPage);
		} catch (error) {
			handleInlineError(error, 'Unable to Delete User');
			throw error;
		}
	}

	function handleModalClose() {
		showUserModal = false;
		showDeleteModal = false;
		isEditMode = false;
		editingUserId = null;
		editingUserData = null;
		deleteData = null;
	}
</script>

<!-- Users Header Actions -->
<div class="flex items-center justify-between mb-6">
	<!-- Search -->
	<SearchInput
		bind:value={searchQuery}
		placeholder="Search users..."
		{loading}
		onSearch={handleSearch}
	/>

	<!-- Add User Button -->
	<button
		onclick={handleAdd}
		class="bg-blue-600 text-white px-4 py-2 rounded-lg text-sm font-medium hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 transition-colors"
	>
		<i class="ti ti-plus text-base mr-2"></i>
		Add User
	</button>
</div>

<!-- Users Table -->
<div class="mb-6">
	<Table
		data={users}
		columns={tableColumns}
		actions={tableActions}
		{loading}
		emptyMessage="No users found. Get started by adding your first user."
	/>
</div>

<!-- Users Pagination -->
{#if pageCount > 1}
	<Pagination
		currentPage={currentPage}
		totalPages={pageCount}
		on:page-change={handlePageChange}
	/>
{/if}

<!-- User Modal -->
{#if showUserModal}
	<UserModal
		{isEditMode}
		userData={editingUserData}
		availableGroups={groups}
		onSave={handleUserSave}
		onClose={handleModalClose}
	/>
{/if}

<!-- Delete Modal -->
{#if showDeleteModal && deleteData}
	<DeleteModal
		title="Delete User"
		itemName={deleteData.name}
		onConfirm={handleDeleteConfirm}
		onClose={handleModalClose}
	/>
{/if}