<script lang="ts">
	import { handleInlineError } from '$lib/utils/errorHandling';
	import type { CredentialReq, CredentialResp } from '$lib/types';

	interface Props {
		isEditMode?: boolean;
		credentialData?: CredentialResp | null;
		onSave: (credentialData: CredentialReq) => void;
		onClose: () => void;
	}

	let { 
		isEditMode = false, 
		credentialData = null, 
		onSave,
		onClose 
	}: Props = $props();

	// Form state
	let formData = $state({
		name: '',
		key_type: '' as 'private_key' | 'password' | '',
		key_data: ''
	});

	let loading = $state(false);

	// Initialize form data when credentialData changes
	$effect(() => {
		if (isEditMode && credentialData) {
			formData = {
				name: credentialData.name || '',
				key_type: credentialData.key_type as 'private_key' | 'password',
				key_data: '' // Don't load existing key data for security
			};
		} else if (!isEditMode) {
			// Reset form for new credential
			formData = {
				name: '',
				key_type: '',
				key_data: ''
			};
		}
	});

	async function handleSubmit() {
		try {
			loading = true;

			const credentialFormData: CredentialReq = {
				name: formData.name,
				key_type: formData.key_type as 'private_key' | 'password',
				key_data: formData.key_data
			};

			await onSave(credentialFormData);
		} catch (err) {
			handleInlineError(err, isEditMode ? 'Unable to Update Credential' : 'Unable to Create Credential');
		} finally {
			loading = false;
		}
	}

	function handleClose() {
		onClose();
	}

	// Close on Escape key
	function handleKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape') {
			handleClose();
		}
	}
</script>

<svelte:window on:keydown={handleKeydown} />

<!-- Modal Backdrop -->
<div class="fixed inset-0 z-50 flex items-center justify-center bg-gray-900/60" on:click={handleClose}>
	<!-- Modal Content -->
	<div class="bg-white rounded-lg shadow-lg w-full max-w-2xl p-6" on:click|stopPropagation>
		<h3 class="font-bold text-lg mb-4 text-gray-900">
			{isEditMode ? 'Edit Credential' : 'Add New Credential'}
		</h3>


		<form on:submit|preventDefault={handleSubmit}>
			<div class="grid grid-cols-2 gap-4 mb-4">
				<!-- Name -->
				<div>
					<label class="block mb-1 font-medium text-gray-900">Credential Name *</label>
					<input 
						type="text" 
						class="bg-gray-50 border border-gray-300 text-gray-900 text-sm rounded-lg focus:ring-blue-500 focus:border-blue-500 block w-full p-2.5" 
						bind:value={formData.name}
						placeholder="my-ssh-key"
						required 
						disabled={loading}
					/>
				</div>
				
				<!-- Type -->
				<div>
					<label class="block mb-1 font-medium text-gray-900">Type *</label>
					<select 
						class="bg-gray-50 border border-gray-300 text-gray-900 text-sm rounded-lg focus:ring-blue-500 focus:border-blue-500 block w-full p-2.5" 
						bind:value={formData.key_type}
						required
						disabled={loading}
					>
						<option value="">Select type...</option>
						<option value="private_key">SSH Key</option>
						<option value="password">Password</option>
					</select>
				</div>
			</div>
			
			<!-- SSH Key Fields -->
			{#if formData.key_type === 'private_key'}
				<div class="mb-4">
					<label class="block mb-1 font-medium text-gray-900">Private Key *</label>
					<textarea 
						class="bg-gray-50 border border-gray-300 text-gray-900 text-sm rounded-lg focus:ring-blue-500 focus:border-blue-500 block w-full p-2.5 resize-none h-32 font-mono text-xs" 
						bind:value={formData.key_data}
						placeholder="-----BEGIN OPENSSH PRIVATE KEY-----"
						required
						disabled={loading}
					></textarea>
				</div>
			{/if}

			<!-- Password Fields -->
			{#if formData.key_type === 'password'}
				<div class="mb-4">
					<label class="block mb-1 font-medium text-gray-900">Password *</label>
					<input 
						type="password" 
						class="bg-gray-50 border border-gray-300 text-gray-900 text-sm rounded-lg focus:ring-blue-500 focus:border-blue-500 block w-full p-2.5"
						bind:value={formData.key_data}
						placeholder="Enter password"
						required
						disabled={loading}
					/>
				</div>
			{/if}

			<!-- Actions -->
			<div class="flex justify-end gap-2 mt-6">
				<button 
					type="button" 
					class="inline-flex items-center px-5 py-2.5 text-sm font-medium text-gray-700 bg-gray-100 rounded-lg hover:bg-gray-200 disabled:opacity-50" 
					on:click={handleClose}
					disabled={loading}
				>
					Cancel
				</button>
				<button 
					type="submit" 
					class="inline-flex items-center px-5 py-2.5 text-sm font-medium text-white bg-blue-700 rounded-lg hover:bg-blue-800 focus:ring-4 focus:outline-none focus:ring-blue-300 disabled:opacity-50" 
					disabled={loading}
				>
					{#if loading}
						<svg class="animate-spin -ml-1 mr-2 h-4 w-4 text-white" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
							<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
							<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
						</svg>
					{/if}
					{isEditMode ? 'Update' : 'Create'}
				</button>
			</div>
		</form>
	</div>
</div>