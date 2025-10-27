<script lang="ts">
	import { goto } from '$app/navigation';
	import { browser } from '$app/environment';
	import { page } from '$app/state';
	import type { PageData } from './$types';
	import PageHeader from '$lib/components/shared/PageHeader.svelte';
	import SearchInput from '$lib/components/shared/SearchInput.svelte';
	import Table from '$lib/components/shared/Table.svelte';
	import Pagination from '$lib/components/shared/Pagination.svelte';
	import CredentialModal from '$lib/components/credentials/CredentialModal.svelte';
	import DeleteModal from '$lib/components/shared/DeleteModal.svelte';
	import { apiClient } from '$lib/apiClient';
	import type { CredentialResp, CredentialReq } from '$lib/types';
	import { DEFAULT_PAGE_SIZE } from '$lib/constants';
	import Header from '$lib/components/shared/Header.svelte';
	import { handleInlineError, showSuccess } from '$lib/utils/errorHandling';
	import { IconPlus } from '@tabler/icons-svelte';
	import { formatDateTime } from '$lib/utils';

	let { data }: { data: PageData } = $props();

	// State
	let credentials = $state(data.credentials);
	let totalCount = $state(data.totalCount);
	let pageCount = $state(data.pageCount);
	let currentPage = $state(data.currentPage);
	let searchQuery = $state(data.searchQuery);
	let permissions = $state(data.permissions);
	let loading = $state(false);
	let showModal = $state(false);
	let isEditMode = $state(false);
	let editingCredentialId = $state<string | null>(null);
	let editingCredentialData = $state<CredentialResp | null>(null);
	let showDeleteModal = $state(false);
	let deleteCredentialId = $state<string | null>(null);
	let deleteCredentialName = $state('');

	// Table configuration
	let tableColumns = [
		{
			key: 'name',
			header: 'Name',
			sortable: true,
			render: (_value: any, credential: CredentialResp) => `
				<div class="flex items-center">
					<div class="w-10 h-10 rounded-lg flex items-center justify-center mr-3 ${
						credential.key_type === 'private_key' ? 'bg-success-100' : 'bg-warning-100'
					}">
						${
							credential.key_type === 'private_key'
								? '<svg class="w-5 h-5 text-success-600" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z"></path></svg>'
								: '<svg class="w-5 h-5 text-warning-600" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"></path></svg>'
						}
					</div>
					<div>
						<div class="text-sm font-medium text-gray-900">${credential.name}</div>
						<div class="text-sm text-gray-500">${credential.id}</div>
					</div>
				</div>
			`
		},
		{
			key: 'key_type',
			header: 'Type',
			sortable: true,
			render: (_value: any, credential: CredentialResp) => `
				<span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
					credential.key_type === 'private_key'
						? 'bg-success-100 text-success-800'
						: 'bg-warning-100 text-warning-800'
				}">${credential.key_type === 'private_key' ? 'SSH Key' : 'Password'}</span>
			`
		},
		{
			key: 'last_accessed',
			header: 'Last Accessed',
			sortable: true,
			render: (_value: any, credential: CredentialResp) => {
				if (!credential.last_accessed || credential.last_accessed === '0001-01-01T00:00:00Z') return 'Never';
				return formatDateTime(credential.last_accessed);
			}
		}
	];

	let tableActions = $derived(() => {
		const actionsList = [];
		
		if (permissions.canUpdate) {
			actionsList.push({
				label: 'Edit',
				onClick: (credential: CredentialResp) => handleEdit(credential.id),
				className: 'text-primary-600 hover:text-primary-800'
			});
		}
		
		if (permissions.canDelete) {
			actionsList.push({
				label: 'Delete',
				onClick: (credential: CredentialResp) => handleDelete(credential.id),
				className: 'text-danger-600 hover:text-danger-800'
			});
		}
		
		return actionsList;
	});

	// Functions
	async function fetchCredentials(filter: string = '', pageNumber: number = 1) {
		if (!browser) return;
		
		loading = true;
		try {
			const response = await apiClient.credentials.list(data.namespace, {
				page: pageNumber,
				count_per_page: DEFAULT_PAGE_SIZE,
				filter: filter
			});

			credentials = response.credentials || [];
			totalCount = response.total_count || 0;
			pageCount = response.page_count || 1;
		} catch (error) {
			handleInlineError(error, 'Unable to Load Credentials List');
		} finally {
			loading = false;
		}
	}

	function handleSearch(query: string) {
		searchQuery = query;
		fetchCredentials(query);
	}

	function handlePageChange(event: CustomEvent<{ page: number }>) {
		currentPage = event.detail.page;
		fetchCredentials('', currentPage);
	}

	function handleAdd() {
		isEditMode = false;
		editingCredentialId = null;
		editingCredentialData = null;
		showModal = true;
	}

	async function handleEdit(credentialId: string) {
		try {
			loading = true;
			const credential = await apiClient.credentials.getById(data.namespace, credentialId);
			
			isEditMode = true;
			editingCredentialId = credentialId;
			editingCredentialData = credential;
			showModal = true;
		} catch (error) {
			handleInlineError(error, 'Unable to Load Credential Details');
		} finally {
			loading = false;
		}
	}


	function handleDelete(credentialId: string) {
		const credential = credentials.find(c => c.id === credentialId);
		if (credential) {
			deleteCredentialId = credentialId;
			deleteCredentialName = credential.name;
			showDeleteModal = true;
		}
	}

	async function confirmDelete() {
		if (!deleteCredentialId) return;

		try {
			await apiClient.credentials.delete(data.namespace, deleteCredentialId);
			showSuccess('Credential Deleted', `Successfully deleted credential "${deleteCredentialName}"`);
			closeDeleteModal(); // Close modal after successful deletion
			await fetchCredentials();
		} catch (error) {
			handleInlineError(error, 'Unable to Delete Credential');
		}
	}

	function closeDeleteModal() {
		showDeleteModal = false;
		deleteCredentialId = null;
		deleteCredentialName = '';
	}

	async function handleCredentialSave(credentialData: CredentialReq) {
		try {
			if (isEditMode && editingCredentialId) {
				await apiClient.credentials.update(data.namespace, editingCredentialId, credentialData);
				showSuccess('Credential Updated', `Successfully updated credential "${credentialData.name}"`);
			} else {
				await apiClient.credentials.create(data.namespace, credentialData);
				showSuccess('Credential Created', `Successfully created credential "${credentialData.name}"`);
			}
			showModal = false;
			await fetchCredentials();
		} catch (error) {
			handleInlineError(error, isEditMode ? 'Unable to Update Credential' : 'Unable to Create Credential');
		}
	}

	function handleModalClose() {
		showModal = false;
		isEditMode = false;
		editingCredentialId = null;
		editingCredentialData = null;
	}


</script>

<svelte:head>
  <title>Credentials - {page.params.namespace} - Flowctl</title>
</svelte:head>

<Header breadcrumbs={[
  { label: page.params.namespace!, url: `/view/${page.params.namespace}/flows` },
  { label: "Credentials" }
]}>
  {#snippet children()}
    <SearchInput
      bind:value={searchQuery}
      placeholder="Search credentials..."
      {loading}
      onSearch={handleSearch}
    />
  {/snippet}
</Header>

<div class="p-12">
	<!-- Page Header -->
	<PageHeader 
		title="Credentials"
		subtitle="Manage SSH keys, passwords, and other authentication credentials"
		actions={permissions.canCreate ? [
			{
				label: 'Add Credential',
				onClick: handleAdd,
				variant: 'primary',
				IconComponent: IconPlus,
				iconSize: 16
			}
		] : []}
	/>

	<!-- Credentials Table -->
	<div class="pt-6">
		<Table
			data={credentials}
			columns={tableColumns}
			actions={tableActions()}
			{loading}
			emptyMessage="No credentials found. Get started by adding your first credential."
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

<!-- Credential Modal -->
{#if showModal}
	<CredentialModal
		{isEditMode}
		credentialData={editingCredentialData}
		onSave={handleCredentialSave}
		onClose={handleModalClose}
	/>
{/if}

<!-- Delete Modal -->
{#if showDeleteModal}
	<DeleteModal
		title="Delete Credential"
		description="Deleting this credential will remove any nodes using it"
		itemName={deleteCredentialName}
		onConfirm={confirmDelete}
		onClose={closeDeleteModal}
	/>
{/if}

