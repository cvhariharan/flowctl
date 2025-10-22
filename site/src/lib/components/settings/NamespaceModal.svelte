<script lang="ts">
	import { handleInlineError } from '$lib/utils/errorHandling';
	import type { NamespaceResp } from '$lib/types';

	let {
		isEditMode = false,
		namespaceData = null,
		onSave,
		onClose
	}: {
		isEditMode: boolean;
		namespaceData: NamespaceResp | null;
		onSave: (data: any) => Promise<void>;
		onClose: () => void;
	} = $props();

	// Form state
	let name = $state(namespaceData?.name || '');
	let saving = $state(false);

	async function handleSubmit(event: Event) {
		event.preventDefault();
		
		saving = true;

		try {
			await onSave({
				name: name.trim()
			});
		} catch (err) {
			handleInlineError(err, isEditMode ? 'Unable to Update Namespace' : 'Unable to Create Namespace');
		} finally {
			saving = false;
		}
	}

	function handleClose() {
		if (!saving) {
			onClose();
		}
	}

	// Handle escape key
	function handleKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape' && !saving) {
			onClose();
		}
	}
</script>

<svelte:window on:keydown={handleKeydown} />

<!-- Modal Background -->
<div class="fixed inset-0 z-50 flex items-center justify-center bg-gray-900/60" onclick={handleClose} role="dialog" aria-modal="true">
	<!-- Modal Content -->
	<div 
		class="bg-white rounded-lg shadow-lg w-full max-w-lg p-6 m-4"
		onclick={(e) => e.stopPropagation()}
		role="document"
	>
		<h3 class="font-bold text-lg mb-4 text-gray-900">
			{isEditMode ? 'Edit Namespace' : 'Add New Namespace'}
		</h3>

		<form onsubmit={handleSubmit}>
			<!-- Name Field -->
			<div class="mb-4">
				<label for="name" class="block mb-1 font-medium text-gray-900">Name</label>
				<input
					type="text"
					id="name"
					bind:value={name}
					required
					disabled={saving}
					maxlength="150"
					class="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-primary-500 disabled:bg-gray-100 disabled:cursor-not-allowed"
					placeholder="Enter namespace name"
				/>
				<p class="mt-1 text-xs text-gray-500">
					Namespace names should be unique and descriptive.
				</p>
			</div>

			<!-- Action Buttons -->
			<div class="flex justify-end gap-2 mt-6">
				<button
					type="button"
					onclick={handleClose}
					disabled={saving}
					class="px-5 py-2.5 text-sm font-medium text-gray-700 bg-gray-100 rounded-lg hover:bg-gray-200 disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer"
				>
					Cancel
				</button>
				<button
					type="submit"
					disabled={saving || !name.trim()}
					class="px-5 py-2.5 text-sm font-medium text-white bg-primary-500 rounded-lg hover:bg-primary-600 disabled:opacity-50 disabled:cursor-not-allowed flex items-center cursor-pointer"
				>
					{#if saving}
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