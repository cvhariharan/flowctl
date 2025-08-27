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
						credential.key_type === 'private_key' ? 'bg-green-100' : 'bg-yellow-100'
					}">
						<i class="ti ${
							credential.key_type === 'private_key' ? 'ti-shield-check text-green-600' : 'ti-lock text-yellow-600'
						}"></i>
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
						? 'bg-green-100 text-green-800' 
						: 'bg-yellow-100 text-yellow-800'
				}">${credential.key_type === 'private_key' ? 'SSH Key' : 'Password'}</span>
			`
		},
		{
			key: 'last_accessed',
			header: 'Last Accessed',
			sortable: true,
			render: (_value: any, credential: CredentialResp) => formatDate(credential.last_accessed)
		}
	];

	let tableActions = $derived(() => {
		const actionsList = [];
		
		if (permissions.canUpdate) {
			actionsList.push({
				label: 'Edit',
				onClick: (credential: CredentialResp) => handleEdit(credential.id),
				className: 'text-blue-600 hover:text-blue-800'
			});
		}
		
		if (permissions.canDelete) {
			actionsList.push({
				label: 'Delete',
				onClick: (credential: CredentialResp) => handleDelete(credential.id),
				className: 'text-red-600 hover:text-red-800'
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
			throw error;
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
			throw error;
		}
	}

	function handleModalClose() {
		showModal = false;
		isEditMode = false;
		editingCredentialId = null;
		editingCredentialData = null;
	}


	function formatDate(dateString: string | null): string {
		if (!dateString) return 'Never';
		const date = new Date(dateString);
		const now = new Date();
		const diffMs = now.getTime() - date.getTime();
		const diffHours = Math.floor(diffMs / (1000 * 60 * 60));
		const diffDays = Math.floor(diffHours / 24);

		if (diffHours < 1) return 'Less than 1 hour ago';
		if (diffHours < 24) return `${diffHours} hours ago`;
		if (diffDays < 7) return `${diffDays} days ago`;
		return date.toLocaleDateString();
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
				icon: '<i class="ti ti-plus"></i>'
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

