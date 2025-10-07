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
	import { IconPlus } from '@tabler/icons-svelte';

	let {
		users: initialUsers,
		totalCount: initialTotalCount,
		pageCount: initialPageCount,
		groups
	}: {
		users: UserWithGroups[];
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
			render: (_value: any, user: User) => {
				const firstLetter = user.name.charAt(0).toUpperCase();
				const colors = ['bg-danger-100 text-danger-600', 'bg-primary-100 text-primary-600', 'bg-success-100 text-success-600', 'bg-warning-100 text-warning-600', 'bg-primary-100 text-primary-600', 'bg-pink-100 text-pink-600', 'bg-indigo-100 text-indigo-600'];
				const colorIndex = user.name.charCodeAt(0) % colors.length;
				const colorClass = colors[colorIndex];

				return `
					<div class="flex items-center">
						<div class="w-10 h-10 rounded-lg flex items-center justify-center mr-3 ${colorClass} font-medium text-sm">
							${firstLetter}
						</div>
						<div>
							<div class="text-sm font-medium text-gray-900">${user.name}</div>
							<div class="text-sm text-gray-500">${user.username}</div>
						</div>
					</div>
				`;
			}
		},
		{
			key: 'groups',
			header: 'Groups',
			render: (_value: any, user: UserWithGroups) => {
				const userGroups = user.groups || [];
				if (userGroups.length === 0) {
					return '<span class="text-gray-400 text-sm">No groups</span>';
				}

				let html = '<div class="flex flex-wrap gap-1 items-center">';

				// Show first 3 groups
				userGroups.slice(0, 3).forEach((group: any) => {
					html += `<span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-primary-100 text-primary-800">${group.name}</span>`;
				});

				// Show +N more if there are more than 3
				if (userGroups.length > 3) {
					html += `<span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-gray-100 text-gray-800">+${userGroups.length - 3}</span>`;
				}

				html += '</div>';
				return html;
			}
		},
		{
			key: 'actions',
			header: 'Actions',
			render: (_value: any, user: UserWithGroups) => {
				// Don't show actions for superuser role (reserved users)
				if (user.role === 'superuser') {
					return '<span class="text-gray-400 text-sm">Reserved</span>';
				}

				return `
					<div class="flex items-center gap-3">
						<button
							data-action="edit"
							data-user-id="${user.id}"
							class="text-primary-600 hover:text-primary-800 text-sm font-medium"
						>
							Edit
						</button>
						<button
							data-action="delete"
							data-user-id="${user.id}"
							data-user-name="${user.name}"
							class="text-danger-600 hover:text-danger-800 text-sm font-medium"
						>
							Delete
						</button>
					</div>
				`;
			}
		}
	];

	// Event delegation for action buttons
	function handleTableClick(event: MouseEvent) {
		const target = event.target as HTMLElement;
		const button = target.closest('button[data-action]');

		if (!button) return;

		const action = button.getAttribute('data-action');
		const userId = button.getAttribute('data-user-id');

		if (action === 'edit' && userId) {
			handleEdit(userId);
		} else if (action === 'delete' && userId) {
			const userName = button.getAttribute('data-user-name') || '';
			handleDelete(userId, userName);
		}
	}

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
		class="bg-primary-500 text-white px-4 py-2 rounded-lg text-sm font-medium hover:bg-primary-600 focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2 transition-colors flex items-center"
	>
		<IconPlus class="mr-2" size={16} />
		Add User
	</button>
</div>

<!-- Users Table -->
<div class="mb-6" onclick={handleTableClick}>
	<Table
		data={users}
		columns={tableColumns}
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