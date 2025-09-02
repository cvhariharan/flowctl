<script lang="ts">
	import { handleInlineError } from '$lib/utils/errorHandling';
	import type { FlowSecretReq, FlowSecretResp } from '$lib/types';

	interface Props {
		isEditMode?: boolean;
		secretData?: FlowSecretResp | null;
		onSave: (secretData: FlowSecretReq) => void;
		onClose: () => void;
	}

	let { 
		isEditMode = false, 
		secretData = null, 
		onSave,
		onClose 
	}: Props = $props();

	// Form state
	let formData = $state({
		key: '',
		value: '',
		description: ''
	});

	let loading = $state(false);

	// Initialize form data when secretData changes
	$effect(() => {
		if (isEditMode && secretData) {
			formData = {
				key: secretData.key || '',
				value: '', // Don't load existing secret value for security
				description: secretData.description || ''
			};
		} else if (!isEditMode) {
			// Reset form for new secret
			formData = {
				key: '',
				value: '',
				description: ''
			};
		}
	});

	async function handleSubmit() {
		try {
			loading = true;

			const secretFormData: FlowSecretReq = {
				key: formData.key,
				value: formData.value,
				description: formData.description || undefined
			};

			await onSave(secretFormData);
		} catch (error) {
			handleInlineError(error);
		} finally {
			loading = false;
		}
	}
</script>

<!-- Modal overlay -->
<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="fixed inset-0 z-50 flex items-center justify-center bg-gray-900/60" onclick={onClose} role="dialog" aria-modal="true">
	<!-- Modal content -->
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div class="bg-white rounded-lg shadow-xl w-full max-w-md mx-4" onclick={(e) => e.stopPropagation()}>
		<div class="p-6">
			<h2 class="text-xl font-semibold mb-4">
				{isEditMode ? 'Edit Secret' : 'Add New Secret'}
			</h2>
			
			<form onsubmit={handleSubmit} class="space-y-4">
				<!-- Secret Key -->
				<div>
					<label for="key" class="block text-sm font-medium text-gray-700 mb-1">
						Key <span class="text-red-500">*</span>
					</label>
					<input
						type="text"
						id="key"
						bind:value={formData.key}
						required
						disabled={loading}
						class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
						placeholder="SECRET_KEY"
					/>
				</div>

				<!-- Secret Value -->
				<div>
					<label for="value" class="block text-sm font-medium text-gray-700 mb-1">
						Value <span class="text-red-500">*</span>
					</label>
					<input
						type="password"
						id="value"
						bind:value={formData.value}
						required
						disabled={loading}
						class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
						placeholder="Enter secret value"
					/>
				</div>

				<!-- Description -->
				<div>
					<label for="description" class="block text-sm font-medium text-gray-700 mb-1">
						Description
					</label>
					<textarea
						id="description"
						bind:value={formData.description}
						disabled={loading}
						rows="3"
						class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
						placeholder="Optional description"
					></textarea>
				</div>

				<!-- Action buttons -->
				<div class="flex justify-end gap-2 pt-4">
					<button
						type="button"
						onclick={onClose}
						disabled={loading}
						class="px-4 py-2 text-gray-600 bg-gray-100 rounded hover:bg-gray-200 disabled:opacity-50"
					>
						Cancel
					</button>
					<button
						type="submit"
						disabled={loading || !formData.key || !formData.value}
						class="px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
					>
						{loading ? 'Saving...' : isEditMode ? 'Update' : 'Create'}
					</button>
				</div>
			</form>
		</div>
	</div>
</div>