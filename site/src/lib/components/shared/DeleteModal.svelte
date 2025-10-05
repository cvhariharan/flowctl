<script lang="ts">
	import { handleInlineError } from '$lib/utils/errorHandling';
	import { IconAlertTriangle } from '@tabler/icons-svelte';

	let {
		title,
		itemName,
		description = null,
		onConfirm,
		onClose
	}: {
		title: string;
		itemName: string;
		description?: string | null;
		onConfirm: () => Promise<void>;
		onClose: () => void;
	} = $props();

	let deleting = $state(false);

	async function handleConfirm() {
		deleting = true;

		try {
			await onConfirm();
		} catch (err) {
			handleInlineError(err, `Unable to Delete ${itemName}`);
		} finally {
			deleting = false;
		}
	}

	function handleClose() {
		if (!deleting) {
			onClose();
		}
	}

	// Handle escape key
	function handleKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape' && !deleting) {
			onClose();
		}
	}
</script>

<svelte:window on:keydown={handleKeydown} />

<!-- Modal Background -->
<div class="fixed inset-0 z-50 flex items-center justify-center bg-gray-900/60" onclick={handleClose} role="dialog" aria-modal="true">
	<!-- Modal Content -->
	<div 
		class="bg-white rounded-lg shadow-lg w-full max-w-md p-6 m-4"
		onclick={(e) => e.stopPropagation()}
		role="document"
	>
		<div class="flex items-center mb-4">
			<div class="w-12 h-12 bg-danger-100 rounded-lg flex items-center justify-center mr-4">
				<IconAlertTriangle class="text-2xl text-danger-600" size={24} />
			</div>
			<div>
				<h3 class="text-lg font-semibold text-gray-900">
					{title}
				</h3>
				<p class="text-sm text-gray-600">This action cannot be undone.</p>
			</div>
		</div>

		<p class="text-gray-700 mb-6">
			Are you sure you want to delete "<span class="font-medium">{itemName}</span>"?
		</p>

		{#if description}
			<p class="text-gray-700 mb-6 font-bold">
				{description}
			</p>
		{/if}


		<div class="flex justify-end gap-2">
			<button
				onclick={handleClose}
				disabled={deleting}
				class="px-5 py-2.5 text-sm font-medium text-gray-700 bg-gray-100 rounded-lg hover:bg-gray-200 disabled:opacity-50 disabled:cursor-not-allowed"
			>
				Cancel
			</button>
			<button
				onclick={handleConfirm}
				disabled={deleting}
				class="px-5 py-2.5 text-sm font-medium text-white bg-danger-500 rounded-lg hover:bg-danger-600 disabled:opacity-50 disabled:cursor-not-allowed flex items-center"
			>
				{#if deleting}
					<svg class="animate-spin -ml-1 mr-2 h-4 w-4 text-white" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
						<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
						<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
					</svg>
				{/if}
				Delete
			</button>
		</div>
	</div>
</div>