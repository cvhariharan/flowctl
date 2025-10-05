<script lang="ts">
	import { handleInlineError } from '$lib/utils/errorHandling';
	import type { CredentialResp, NodeReq, NodeResp } from '$lib/types';

	interface Props {
		isEditMode?: boolean;
		nodeData?: NodeResp | null;
		credentials: CredentialResp[];
		onSave: (nodeData: NodeReq) => void;
		onClose: () => void;
	}

	let { 
		isEditMode = false, 
		nodeData = null, 
		credentials,
		onSave,
		onClose 
	}: Props = $props();

	// Form state
	let formData = $state({
		name: '',
		hostname: '',
		port: 22,
		username: '',
		os_family: '',
		connection_type: 'ssh',
		auth: {
			credential_id: '',
			method: ''
		},
		tags: [] as string[],
		tagsString: ''
	});

	let loading = $state(false);

	// Initialize form data when nodeData changes
	$effect(() => {
		if (isEditMode && nodeData) {
			formData.name = nodeData.name || '';
			formData.hostname = nodeData.hostname || '';
			formData.port = nodeData.port || 22;
			formData.username = nodeData.username || '';
			formData.os_family = nodeData.os_family || '';
			formData.connection_type = nodeData.connection_type || 'ssh';
			formData.auth.credential_id = nodeData.auth?.credential_id || '';
			formData.auth.method = nodeData.auth?.method || '';
			formData.tags = nodeData.tags || [];
			formData.tagsString = (nodeData.tags || []).join(', ');
		} else if (!isEditMode) {
			// Reset form for new node
			formData.name = '';
			formData.hostname = '';
			formData.port = 22;
			formData.username = '';
			formData.os_family = '';
			formData.connection_type = 'ssh';
			formData.auth.credential_id = '';
			formData.auth.method = '';
			formData.tags = [];
			formData.tagsString = '';
		}
	});

	function onCredentialChange() {
		if (formData.auth.credential_id) {
			const selectedCredential = credentials.find(c => c.id === formData.auth.credential_id);
			if (selectedCredential) {
				formData.auth.method = selectedCredential.key_type;
			}
		} else {
			formData.auth.method = '';
		}
	}

	function getAuthMethodDisplay(method: string) {
		switch (method) {
			case 'private_key':
				return 'SSH Key';
			case 'password':
				return 'Password';
			default:
				return '';
		}
	}

	async function handleSubmit() {
		try {
			loading = true;

			const tags = formData.tagsString
				.split(',')
				.map(tag => tag.trim())
				.filter(tag => tag.length > 0);

			const nodeFormData: NodeReq = {
				name: formData.name,
				hostname: formData.hostname,
				port: formData.port,
				username: formData.username,
				os_family: formData.os_family,
				connection_type: formData.connection_type,
				tags: tags,
				auth: {
					credential_id: formData.auth.credential_id,
					method: formData.auth.method
				}
			};

			await onSave(nodeFormData);
		} catch (err) {
			handleInlineError(err, isEditMode ? 'Unable to Update Node' : 'Unable to Create Node');
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
<div class="fixed inset-0 z-50 flex items-center justify-center bg-gray-900/60 p-4" on:click={handleClose}>
	<!-- Modal Content -->
	<div class="bg-white rounded-lg shadow-lg w-full max-w-lg max-h-[90vh] overflow-y-auto" on:click|stopPropagation>
		<div class="p-6">
		<h3 class="font-bold text-lg mb-4 text-gray-900">
			{isEditMode ? 'Edit Node' : 'Add Node'}
		</h3>


		<form on:submit|preventDefault={handleSubmit}>
			<!-- Name -->
			<div class="mb-4">
				<label class="block mb-1 font-medium text-gray-900">Name</label>
				<input 
					type="text" 
					class="bg-gray-50 border border-gray-300 text-gray-900 text-sm rounded-lg focus:ring-primary-500 focus:border-primary-500 block w-full p-2.5" 
					bind:value={formData.name}
					required 
					disabled={loading}
				/>
			</div>

			<!-- Hostname -->
			<div class="mb-4">
				<label class="block mb-1 font-medium text-gray-900">Hostname</label>
				<input 
					type="text" 
					class="bg-gray-50 border border-gray-300 text-gray-900 text-sm rounded-lg focus:ring-primary-500 focus:border-primary-500 block w-full p-2.5" 
					bind:value={formData.hostname}
					required 
					disabled={loading}
				/>
			</div>

			<!-- Port -->
			<div class="mb-4">
				<label class="block mb-1 font-medium text-gray-900">Port</label>
				<input 
					type="number" 
					class="bg-gray-50 border border-gray-300 text-gray-900 text-sm rounded-lg focus:ring-primary-500 focus:border-primary-500 block w-full p-2.5" 
					bind:value={formData.port}
					min="1" 
					max="65535" 
					required 
					disabled={loading}
				/>
			</div>

			<!-- Username -->
			<div class="mb-4">
				<label class="block mb-1 font-medium text-gray-900">Username</label>
				<input 
					type="text" 
					class="bg-gray-50 border border-gray-300 text-gray-900 text-sm rounded-lg focus:ring-primary-500 focus:border-primary-500 block w-full p-2.5" 
					bind:value={formData.username}
					required 
					disabled={loading}
				/>
			</div>

			<!-- OS Family -->
			<div class="mb-4">
				<label class="block mb-1 font-medium text-gray-900">OS Family</label>
				<select 
					class="bg-gray-50 border border-gray-300 text-gray-900 text-sm rounded-lg focus:ring-primary-500 focus:border-primary-500 block w-full p-2.5" 
					bind:value={formData.os_family}
					required
					disabled={loading}
				>
					<option value="">Select OS</option>
					<option value="linux">Linux</option>
					<option value="windows">Windows</option>
				</select>
			</div>

			<!-- Connection Type -->
			<div class="mb-4">
				<label class="block mb-1 font-medium text-gray-900">Connection Type</label>
				<select 
					class="bg-gray-50 border border-gray-300 text-gray-900 text-sm rounded-lg focus:ring-primary-500 focus:border-primary-500 block w-full p-2.5" 
					bind:value={formData.connection_type}
					required
					disabled={loading}
				>
					<option value="">Select connection type</option>
					<option value="ssh">SSH</option>
					<option value="qssh">QSSH</option>
				</select>
			</div>

			<!-- Credential -->
			<div class="mb-4">
				<label class="block mb-1 font-medium text-gray-900">Credential</label>
				<select 
					class="bg-gray-50 border border-gray-300 text-gray-900 text-sm rounded-lg focus:ring-primary-500 focus:border-primary-500 block w-full p-2.5" 
					bind:value={formData.auth.credential_id}
					on:change={onCredentialChange}
					required
					disabled={loading}
				>
					<option value="">Select credential</option>
					{#each credentials as credential}
						<option value={credential.id}>
							{credential.name} ({credential.key_type})
						</option>
					{/each}
				</select>
			</div>

			<!-- Tags -->
			<div class="mb-4">
				<label class="block mb-1 font-medium text-gray-900">Tags (comma-separated)</label>
				<input 
					type="text" 
					class="bg-gray-50 border border-gray-300 text-gray-900 text-sm rounded-lg focus:ring-primary-500 focus:border-primary-500 block w-full p-2.5" 
					bind:value={formData.tagsString}
					placeholder="production, web, east" 
					disabled={loading}
				/>
			</div>

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
					class="inline-flex items-center px-5 py-2.5 text-sm font-medium text-white bg-primary-500 rounded-lg hover:bg-primary-600 focus:ring-4 focus:outline-none focus:ring-primary-300 disabled:opacity-50"
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
</div>