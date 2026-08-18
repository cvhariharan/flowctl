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
	import IconPlus from '@tabler/icons-svelte/icons/plus';

	let {
		users: initialUsers,
		totalCount: initialTotalCount,
		pageCount: initialPageCount,
		groups,
		refreshTrigger
	}: {
		users: UserWithGroups[];
		totalCount: number;
		pageCount: number;
		groups: Group[];
		refreshTrigger: boolean;
	} = $props();

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
			render: (_value: any, user: User) => {
				const firstLetter = user.name.charAt(0).toUpperCase();
				const c = avatarColors[user.name.charCodeAt(0) % avatarColors.length];

				return `
					<div class="name-cell">
						<div class="avatar" style="background:${c.bg};color:${c.fg}">
							${firstLetter}
						</div>
						<div>
							<div class="name-link" onclick="document.dispatchEvent(new CustomEvent('editUser', {detail: {id: '${user.id}'}}))">${user.name}</div>
							<div class="name-sub">${user.username}</div>
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
					return '<span class="name-sub">No groups</span>';
				}

				let html = '<div class="group-chips">';
				userGroups.slice(0, 3).forEach((group: any) => {
					html += `<span class="group-chip">${group.name}</span>`;
				});
				if (userGroups.length > 3) {
					html += `<span class="group-chip extra">+${userGroups.length - 3}</span>`;
				}
				html += '</div>';
				return html;
			}
		}
	];

	let tableActions = [
		{
			label: 'Edit',
			onClick: (user: UserWithGroups) => handleEdit(user.id),
			visible: (user: UserWithGroups) => user.role !== 'superuser',
		},
		{
			label: 'Delete',
			onClick: (user: UserWithGroups) => handleDelete(user.id, user.name),
			visible: (user: UserWithGroups) => user.role !== 'superuser',
		}
	];

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

	if (browser) {
		document.addEventListener('editUser', ((event: CustomEvent) => {
			handleEdit(event.detail.id);
		}) as EventListener);
	}

	$effect(() => {
		refreshTrigger;
		fetchUsers('', 1);
	});
</script>

<!-- Users Header Actions -->
<div class="hstack mb-4 justify-between">
	<SearchInput
		bind:value={searchQuery}
		placeholder="Search users..."
		{loading}
		onSearch={handleSearch}
	/>

	<button onclick={handleAdd}>
		<IconPlus size={16} />
		Add User
	</button>
</div>

<!-- Users Table -->
<div class="mb-4">
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

<style>
	.mb-4 { margin-bottom: 1.5rem; }

	:global(.name-cell) {
		display: flex;
		align-items: center;
		gap: 0.75rem;
	}
	:global(.avatar) {
		width: 2.5rem;
		height: 2.5rem;
		border-radius: var(--radius-medium);
		display: flex;
		align-items: center;
		justify-content: center;
		font-weight: 500;
		font-size: 0.875rem;
		flex-shrink: 0;
	}
	:global(.name-link) {
		font-size: 0.875rem;
		font-weight: 500;
		color: var(--foreground);
		cursor: pointer;
	}
	:global(.name-link:hover) {
		color: var(--primary);
	}
	:global(.name-sub) {
		font-size: 0.875rem;
		color: var(--muted-foreground);
	}
	:global(.group-chips) {
		display: flex;
		flex-wrap: wrap;
		gap: 0.25rem;
		align-items: center;
	}
	:global(.group-chip) {
		display: inline-flex;
		align-items: center;
		padding: 0.125rem 0.5rem;
		border-radius: 0.25rem;
		font-size: 0.75rem;
		font-weight: 500;
		background: color-mix(in srgb, var(--primary) 15%, transparent);
		color: var(--primary);
	}
	:global(.group-chip.extra) {
		background: var(--faint);
		color: var(--foreground);
	}
</style>
