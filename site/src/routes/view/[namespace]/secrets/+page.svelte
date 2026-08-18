<script lang="ts">
	import { browser } from '$app/environment';
	import { page } from '$app/state';
	import type { PageData } from './$types';
	import PageHeader from '$lib/components/shared/PageHeader.svelte';
	import Header from '$lib/components/shared/Header.svelte';
	import Table from '$lib/components/shared/Table.svelte';
	import NamespaceSecretsModal from '$lib/components/namespace-secrets/NamespaceSecretsModal.svelte';
	import DeleteModal from '$lib/components/shared/DeleteModal.svelte';
	import { apiClient } from '$lib/apiClient';
	import type { NamespaceSecretReq, NamespaceSecretUpdateReq, NamespaceSecretResp, TableColumn, TableAction } from '$lib/types';
	import { handleInlineError, showSuccess } from '$lib/utils/errorHandling';
	import { formatDateTime } from '$lib/utils';
	import IconPlus from '@tabler/icons-svelte/icons/plus';
	import IconLock from '@tabler/icons-svelte/icons/lock';
	import MutedTextCell from '$lib/components/shared/cells/MutedTextCell.svelte';

	let { data }: { data: PageData } = $props();
	const namespace = $derived(page.params.namespace!);
	const encodedNamespace = $derived(encodeURIComponent(namespace));

	// State
	let secrets = $state<NamespaceSecretResp[]>([]);
	let loading = $state(true);
	let showModal = $state(false);
	let showDeleteModal = $state(false);
	let selectedSecret = $state<NamespaceSecretResp | null>(null);
	let isEditMode = $state(false);

	// Handle the async data from load function
	$effect(() => {
		let cancelled = false;

		// Load secrets
		data.secretsPromise
			.then((result: NamespaceSecretResp[]) => {
				if (!cancelled) {
					secrets = result || [];
					loading = false;
				}
			})
			.catch((err: Error) => {
				if (!cancelled) {
					secrets = [];
					handleInlineError(err, "Unable to Load Secrets");
					loading = false;
				}
			});

		return () => {
			cancelled = true;
		};
	});

	async function fetchSecrets() {
		if (!browser) return;

		loading = true;
		try {
			const response = await apiClient.namespaceSecrets.list(data.namespace);
			secrets = response;
		} catch (error) {
			handleInlineError(error, 'Unable to Load Secrets');
		} finally {
			loading = false;
		}
	}

	function handleAdd() {
		selectedSecret = null;
		isEditMode = false;
		showModal = true;
	}

	function handleEdit(secret: NamespaceSecretResp) {
		selectedSecret = secret;
		isEditMode = true;
		showModal = true;
	}

	function handleDelete(secret: NamespaceSecretResp) {
		selectedSecret = secret;
		showDeleteModal = true;
	}

	async function handleSave(secretData: NamespaceSecretReq | NamespaceSecretUpdateReq) {
		try {
			if (isEditMode && selectedSecret) {
				await apiClient.namespaceSecrets.update(data.namespace, selectedSecret.id, secretData as NamespaceSecretUpdateReq);
				showSuccess('Secret Updated', 'Secret updated successfully');
			} else {
				await apiClient.namespaceSecrets.create(data.namespace, secretData as NamespaceSecretReq);
				showSuccess('Secret Created', 'Secret created successfully');
			}

			showModal = false;
			await fetchSecrets();
		} catch (error) {
			handleInlineError(error, isEditMode ? 'Unable to Update Secret' : 'Unable to Create Secret');
		}
	}

	async function confirmDelete() {
		if (!selectedSecret) return;

		try {
			await apiClient.namespaceSecrets.delete(data.namespace, selectedSecret.id);
			showSuccess('Secret Deleted', 'Secret deleted successfully');
			closeDeleteModal();
			await fetchSecrets();
		} catch (error) {
			handleInlineError(error, 'Unable to Delete Secret');
		}
	}

	function closeDeleteModal() {
		showDeleteModal = false;
		selectedSecret = null;
	}

	function handleModalClose() {
		showModal = false;
		isEditMode = false;
		selectedSecret = null;
	}

	const columns: TableColumn<NamespaceSecretResp>[] = [
		{
			key: 'key',
			header: 'Key',
			sortable: true
		},
		{
			key: 'description',
			header: 'Description',
			component: MutedTextCell,
			componentProps: { lighter: true, truncate: 40 }
		},
		{
			key: 'created_at',
			header: 'Created',
			sortable: true,
			component: MutedTextCell,
			componentProps: { format: (v: string) => formatDateTime(v) }
		}
	];

	const actions: TableAction<NamespaceSecretResp>[] = [
		{
			label: 'Edit',
			onClick: (row: NamespaceSecretResp) => handleEdit(row),
			visible: () => !!data.permissions?.canUpdate
		},
		{
			label: 'Delete',
			onClick: (row: NamespaceSecretResp) => handleDelete(row),
			className: 'text-danger',
			visible: () => !!data.permissions?.canDelete
		}
	];
</script>

<svelte:head>
  <title>Secrets - {page.params.namespace} - Flowctl</title>
</svelte:head>

<Header breadcrumbs={[
  { label: namespace, url: `/view/${encodedNamespace}/flows` },
  { label: "Secrets" }
]}>

</Header>

<div class="page-content">
	<!-- Page Header -->
	<PageHeader
		title="Secrets"
		subtitle="Manage encrypted secrets available to all flows in this namespace."
		actions={data.permissions?.canCreate ? [
			{
				label: 'Add',
				onClick: handleAdd,
				variant: 'primary',
				IconComponent: IconPlus,
				iconSize: 16
			}
		] : []}
	/>

	<Table
		{columns}
		data={secrets}
		{actions}
		{loading}
		emptyMessage="No secrets found. Get started by adding your first secret."
		EmptyIconComponent={IconLock}
		emptyActionLabel={data.permissions?.canCreate ? "Add your first secret" : undefined}
		onEmptyAction={data.permissions?.canCreate ? handleAdd : undefined}
	/>
</div>

<!-- Secret Modal -->
{#if showModal}
	<NamespaceSecretsModal
		{isEditMode}
		secretData={selectedSecret}
		onSave={handleSave}
		onClose={handleModalClose}
	/>
{/if}

<!-- Delete Modal -->
{#if showDeleteModal}
	<DeleteModal
		title="Delete Secret"
		itemName={selectedSecret?.key ?? ''}
		description="This may break flow executions that depend on this secret."
		onConfirm={confirmDelete}
		onClose={closeDeleteModal}
	/>
{/if}
