<script lang="ts">
	import { handleInlineError, showSuccess } from '$lib/utils/errorHandling';
	import { apiClient } from '$lib/apiClient';
	import type { FlowSecretReq, FlowSecretUpdateReq, FlowSecretResp, NamespaceSecretResp } from '$lib/types';
	import SecretsModal from './SecretsModal.svelte';
	import DeleteModal from '../shared/DeleteModal.svelte';
	import { formatDateTime } from '$lib/utils';

	interface Props {
		namespace: string;
		flowId?: string;
		disabled?: boolean;
	}

	let { namespace, flowId, disabled = false }: Props = $props();

	let secrets = $state<FlowSecretResp[]>([]);
	let namespaceSecrets = $state<NamespaceSecretResp[]>([]);
	let loading = $state(false);
	let loadingNamespaceSecrets = $state(false);
	let showModal = $state(false);
	let showDeleteModal = $state(false);
	let selectedSecret = $state<FlowSecretResp | null>(null);
	let isEditMode = $state(false);

	$effect(() => {
		loadNamespaceSecrets();
	});

	$effect(() => {
		if (flowId && !disabled) {
			loadSecrets();
		}
	});

	async function loadNamespaceSecrets() {
		try {
			loadingNamespaceSecrets = true;
			namespaceSecrets = await apiClient.namespaceSecrets.list(namespace);
		} catch (error) {
			namespaceSecrets = [];
		} finally {
			loadingNamespaceSecrets = false;
		}
	}

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

	async function handleSave(secretData: FlowSecretReq | FlowSecretUpdateReq) {
		if (!flowId) {
			handleInlineError(new Error('Flow must be saved before adding secrets'));
			return;
		}

		try {
			if (isEditMode && selectedSecret) {
				await apiClient.flowSecrets.update(namespace, flowId, selectedSecret.id, secretData as FlowSecretUpdateReq);
				showSuccess('Flow Secret Updated', 'Secret updated successfully');
			} else {
				await apiClient.flowSecrets.create(namespace, flowId, secretData as FlowSecretReq);
				showSuccess('Flow Secret Created', 'Secret created successfully');
			}

			showModal = false;
			await loadSecrets();
		} catch (error) {
			handleInlineError(error, isEditMode ? 'Unable to Update Secret' : 'Unable to Create Secret');
		}
	}

	async function handleDelete() {
		if (!selectedSecret || !flowId) return;

		try {
			await apiClient.flowSecrets.delete(namespace, flowId, selectedSecret.id);
			showSuccess('Flow Secret Deleted', 'Secret deleted successfully');
			showDeleteModal = false;
			await loadSecrets();
		} catch (error) {
			handleInlineError(error);
		}
	}

</script>

<div class="vstack gap-4">
	<div class="hstack justify-between">
		<div>
			<h3>Flow Secrets</h3>
			<p class="text-light">
				{flowId
					? 'Manage encrypted secrets for this flow. Values are never displayed after creation.'
					: 'Save the flow first to add secrets.'
				}
			</p>
		</div>

		{#if flowId && !disabled}
			<button onclick={openCreateModal}>
				Add Secret
			</button>
		{/if}
	</div>

	{#if loading}
		<div class="centered-msg" aria-busy="true">Loading...</div>
	{:else if !flowId}
		<div class="empty-state vstack">
			<svg class="empty-icon" fill="none" viewBox="0 0 24 24" stroke="currentColor">
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
			</svg>
			<h4>No secrets yet</h4>
			<p class="text-lighter">Save the flow first to add secrets.</p>
		</div>
	{:else if secrets.length === 0}
		<div class="empty-state vstack">
			<svg class="empty-icon" fill="none" viewBox="0 0 24 24" stroke="currentColor">
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
			</svg>
			<h4>No secrets yet</h4>
			<p class="text-lighter">Add your first secret to get started.</p>
		</div>
	{:else}
		<div class="secrets-list">
			{#each secrets as secret}
				<div class="secret-item hstack justify-between">
					<div class="hstack gap-2 min-w-0 flex-1">
						<svg class="lock-icon" fill="none" viewBox="0 0 24 24" stroke="currentColor">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
						</svg>
						<div class="min-w-0 flex-1">
							<p class="secret-key">{secret.key}</p>
							{#if secret.description}
								<p class="text-lighter secret-desc">{secret.description}</p>
							{/if}
							<p class="text-lighter secret-date">
								Created: {formatDateTime(secret.created_at)}
							</p>
						</div>
					</div>

					<div class="hstack gap-1">
						<button
							data-variant="secondary"
							class="ghost icon small"
							onclick={() => openEditModal(secret)}
							disabled={disabled}
							title="Edit secret"
						>
							<svg class="icon-sm" fill="none" viewBox="0 0 24 24" stroke="currentColor">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
							</svg>
						</button>
						<button
							data-variant="danger"
							class="ghost icon small"
							onclick={() => openDeleteModal(secret)}
							disabled={disabled}
							title="Delete secret"
						>
							<svg class="icon-sm" fill="none" viewBox="0 0 24 24" stroke="currentColor">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
							</svg>
						</button>
					</div>
				</div>
			{/each}
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
		itemName={selectedSecret?.key || 'secret'}
		description="This may break flow executions that depend on it."
		onConfirm={handleDelete}
		onClose={() => { showDeleteModal = false; }}
	/>
{/if}

<!-- Namespace Secrets Section (Read-only) -->
<div class="ns-secrets-section">
	<div class="mb-4">
		<h3>Namespace Secrets</h3>
		<p class="text-light">
			These secrets are available to all flows in this namespace. Flow secrets with the same key will override these.
		</p>
	</div>

	{#if loadingNamespaceSecrets}
		<div class="centered-msg" aria-busy="true">Loading...</div>
	{:else if namespaceSecrets.length === 0}
		<div class="centered-msg">
			<p class="text-lighter">No namespace secrets configured.</p>
		</div>
	{:else}
		<div class="ns-secrets-list">
			{#each namespaceSecrets as secret}
				<div class="ns-secret-item hstack gap-2">
					<svg class="lock-icon-sm" fill="none" viewBox="0 0 24 24" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
					</svg>
					<div class="min-w-0 flex-1">
						<p class="secret-key">{secret.key}</p>
						{#if secret.description}
							<p class="text-lighter secret-desc">{secret.description}</p>
						{/if}
					</div>
					<span class="badge primary">namespace</span>
				</div>
			{/each}
		</div>
	{/if}
</div>

<style>
	.centered-msg {
		display: flex;
		align-items: center;
		justify-content: center;
		padding: var(--space-8);
	}
	.empty-state {
		text-align: center;
		padding: var(--space-8);
		align-items: center;
		gap: var(--space-2);
	}
	.empty-icon {
		width: 3rem;
		height: 3rem;
		color: var(--muted-foreground);
	}
	.empty-state h4 {
		font-size: var(--text-7);
		font-weight: var(--font-medium);
	}
	.secrets-list {
		background: var(--card);
		border: 1px solid var(--border);
		border-radius: var(--radius-medium);
		overflow: hidden;
	}
	.secrets-list > :not(:last-child) {
		border-bottom: 1px solid var(--border);
	}
	.secret-item {
		padding: var(--space-4);
	}
	.lock-icon {
		width: 1.25rem;
		height: 1.25rem;
		color: var(--muted-foreground);
		flex-shrink: 0;
	}
	.lock-icon-sm {
		width: 1rem;
		height: 1rem;
		color: var(--muted-foreground);
		flex-shrink: 0;
	}
	.secret-key {
		font-size: var(--text-7);
		font-weight: var(--font-medium);
		overflow: hidden;
		text-overflow: ellipsis;
	}
	.secret-desc {
		font-size: var(--text-7);
		overflow: hidden;
		text-overflow: ellipsis;
	}
	.secret-date {
		font-size: var(--text-8);
	}
	.icon-sm {
		width: 1.25rem;
		height: 1.25rem;
	}
	.ns-secrets-section {
		margin-top: var(--space-8);
		padding-top: var(--space-8);
		border-top: 1px solid var(--border);
	}
	.ns-secrets-list {
		background: var(--faint);
		border-radius: var(--radius-medium);
		border: 1px solid var(--border);
	}
	.ns-secrets-list > :not(:last-child) {
		border-bottom: 1px solid var(--border);
	}
	.ns-secret-item {
		padding: var(--space-3) var(--space-4);
	}
</style>
