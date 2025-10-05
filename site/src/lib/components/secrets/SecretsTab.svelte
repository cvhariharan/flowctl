<script lang="ts">
	import { handleInlineError, showSuccess } from '$lib/utils/errorHandling';
	import { apiClient } from '$lib/apiClient';
	import type { FlowSecretReq, FlowSecretResp } from '$lib/types';
	import SecretsModal from './SecretsModal.svelte';
	import DeleteModal from '../shared/DeleteModal.svelte';

	interface Props {
		namespace: string;
		flowId?: string; // Optional for create mode
		disabled?: boolean;
	}

	let { namespace, flowId, disabled = false }: Props = $props();

	// State
	let secrets = $state<FlowSecretResp[]>([]);
	let loading = $state(false);
	let showModal = $state(false);
	let showDeleteModal = $state(false);
	let selectedSecret = $state<FlowSecretResp | null>(null);
	let isEditMode = $state(false);

	// Load secrets when flowId is available (edit mode)
	$effect(() => {
		if (flowId && !disabled) {
			loadSecrets();
		}
	});

	async function loadSecrets() {
		if (!flowId) return;
		
		try {
			loading = true;
			secrets = await apiClient.flowSecrets.list(namespace, flowId);
		} catch (error) {
			handleInlineError(error);
		} finally {
			loading = false;
		}
	}

	function openCreateModal() {
		selectedSecret = null;
		isEditMode = false;
		showModal = true;
	}

	function openEditModal(secret: FlowSecretResp) {
		selectedSecret = secret;
		isEditMode = true;
		showModal = true;
	}

	function openDeleteModal(secret: FlowSecretResp) {
		selectedSecret = secret;
		showDeleteModal = true;
	}

	async function handleSave(secretData: FlowSecretReq) {
		if (!flowId) {
			handleInlineError(new Error('Flow must be saved before adding secrets'));
			return;
		}

		try {
			if (isEditMode && selectedSecret) {
				await apiClient.flowSecrets.update(namespace, flowId, selectedSecret.id, secretData);
				showSuccess('Secret updated successfully');
			} else {
				await apiClient.flowSecrets.create(namespace, flowId, secretData);
				showSuccess('Secret created successfully');
			}
			
			showModal = false;
			await loadSecrets();
		} catch (error) {
			throw error; // Let the modal handle the error
		}
	}

	async function handleDelete() {
		if (!selectedSecret || !flowId) return;

		try {
			await apiClient.flowSecrets.delete(namespace, flowId, selectedSecret.id);
			showSuccess('Secret deleted successfully');
			showDeleteModal = false;
			await loadSecrets();
		} catch (error) {
			handleInlineError(error);
		}
	}

	function formatDate(dateString: string): string {
		return new Date(dateString).toLocaleString();
	}
</script>

<div class="space-y-4">
	<div class="flex justify-between items-center">
		<div>
			<h3 class="text-lg font-medium text-gray-900">Flow Secrets</h3>
			<p class="text-sm text-gray-500">
				{flowId 
					? 'Manage encrypted secrets for this flow. Values are never displayed after creation.'
					: 'Save the flow first to add secrets.'
				}
			</p>
		</div>
		
		{#if flowId && !disabled}
			<button
				onclick={openCreateModal}
				class="px-4 py-2 bg-primary-500 text-white rounded-md hover:bg-primary-600 focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2"
			>
				Add Secret
			</button>
		{/if}
	</div>

	{#if loading}
		<div class="flex items-center justify-center py-8">
			<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
		</div>
	{:else if !flowId}
		<div class="text-center py-8">
			<div class="text-gray-500">
				<svg class="mx-auto h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
				</svg>
				<h3 class="mt-2 text-sm font-medium text-gray-900">No secrets yet</h3>
				<p class="mt-1 text-sm text-gray-500">Save the flow first to add secrets.</p>
			</div>
		</div>
	{:else if secrets.length === 0}
		<div class="text-center py-8">
			<div class="text-gray-500">
				<svg class="mx-auto h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
				</svg>
				<h3 class="mt-2 text-sm font-medium text-gray-900">No secrets yet</h3>
				<p class="mt-1 text-sm text-gray-500">Add your first secret to get started.</p>
				<div class="mt-6">
					<button
						onclick={openCreateModal}
						class="inline-flex items-center px-4 py-2 border border-transparent shadow-sm text-sm font-medium rounded-md text-white bg-primary-500 hover:bg-primary-600"
					>
						Add Secret
					</button>
				</div>
			</div>
		</div>
	{:else}
		<div class="bg-white shadow overflow-hidden sm:rounded-md">
			<ul role="list" class="divide-y divide-gray-200">
				{#each secrets as secret}
					<li class="px-4 py-4 flex items-center justify-between">
						<div class="flex-1 min-w-0">
							<div class="flex items-center space-x-3">
								<div class="flex-shrink-0">
									<svg class="h-5 w-5 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
										<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
									</svg>
								</div>
								<div class="min-w-0 flex-1">
									<p class="text-sm font-medium text-gray-900 truncate">
										{secret.key}
									</p>
									{#if secret.description}
										<p class="text-sm text-gray-500 truncate">
											{secret.description}
										</p>
									{/if}
									<p class="text-xs text-gray-400">
										Created: {formatDate(secret.created_at)}
									</p>
								</div>
							</div>
						</div>
						
						<div class="flex items-center space-x-2">
							<button
								onclick={() => openEditModal(secret)}
								disabled={disabled}
								class="text-primary-600 hover:text-primary-500 disabled:opacity-50 disabled:cursor-not-allowed"
								title="Edit secret"
							>
								<svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
								</svg>
							</button>
							<button
								onclick={() => openDeleteModal(secret)}
								disabled={disabled}
								class="text-danger-600 hover:text-danger-500 disabled:opacity-50 disabled:cursor-not-allowed"
								title="Delete secret"
							>
								<svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
								</svg>
							</button>
						</div>
					</li>
				{/each}
			</ul>
		</div>
	{/if}
</div>

<!-- Modals -->
{#if showModal}
	<SecretsModal
		{isEditMode}
		secretData={selectedSecret}
		onSave={handleSave}
		onClose={() => { showModal = false; }}
	/>
{/if}

{#if showDeleteModal}
	<DeleteModal
		title="Delete Secret"
		message="Are you sure you want to delete this secret? This action cannot be undone and may break flow executions that depend on it."
		onConfirm={handleDelete}
		onCancel={() => { showDeleteModal = false; }}
	/>
{/if}