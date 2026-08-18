<script lang="ts">
	import { handleInlineError } from '$lib/utils/errorHandling';
	import { autofocus } from '$lib/utils/autofocus';
	import IconAlertTriangle from '@tabler/icons-svelte/icons/alert-triangle';

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
	let dialogEl: HTMLDialogElement;

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
			dialogEl?.close();
			onClose();
		}
	}

	$effect(() => {
		if (dialogEl) {
			dialogEl.showModal();
		}
	});
</script>

<dialog bind:this={dialogEl} onclose={handleClose}>
	<header>
		<div class="hstack gap-4">
			<div class="icon-box danger" style="width:3rem;height:3rem">
				<IconAlertTriangle size={24} />
			</div>
			<div>
				<h3>{title}</h3>
				<p class="text-sm text-light">This action cannot be undone.</p>
			</div>
		</div>
	</header>

	<section>
		<p>
			Are you sure you want to delete "<strong>{itemName}</strong>"?
		</p>

		{#if description}
			<p><strong>{description}</strong></p>
		{/if}
	</section>

	<footer>
		<button
			onclick={handleClose}
			disabled={deleting}
			data-variant="secondary"
			use:autofocus
		>
			Cancel
		</button>
		<button
			onclick={handleConfirm}
			disabled={deleting}
			data-variant="danger"
		>
			{#if deleting}
				<span aria-busy="true"></span>
			{/if}
			Delete
		</button>
	</footer>
</dialog>
