<script lang="ts">
	import { autofocus } from '$lib/utils/autofocus';
import type { UserSchedule } from '$lib/types';

	interface Props {
		schedule: UserSchedule;
		onClose: () => void;
	}

	let { schedule, onClose }: Props = $props();

	let dialogEl: HTMLDialogElement;

	$effect(() => {
		if (dialogEl) {
			dialogEl.showModal();
		}
	});
</script>

<dialog bind:this={dialogEl} onclose={onClose}>
	<header>
		<h3>Schedule Details</h3>
	</header>

	<section>
		<!-- Schedule Info Box -->
		<article class="card p-4 mb-4" style="background: var(--faint)">
			{#if schedule.name}
				<div style="margin-bottom: 1rem">
					<div class="text-xs font-medium text-lighter" style="text-transform: uppercase; margin-bottom: 0.25rem">Name</div>
					<div>{schedule.name}</div>
				</div>
			{/if}
			<div class="row">
				<div class="col-6">
					<div class="text-xs font-medium text-lighter" style="text-transform: uppercase; margin-bottom: 0.25rem">Cron Expression</div>
					<code>{schedule.cron}</code>
				</div>
				<div class="col-6">
					<div class="text-xs font-medium text-lighter" style="text-transform: uppercase; margin-bottom: 0.25rem">Timezone</div>
					<div>{schedule.timezone}</div>
				</div>
			</div>
		</article>

		<!-- Schedule Inputs -->
		{#if schedule.inputs}
			<details open>
				<summary>Inputs</summary>
				<pre><code>{JSON.stringify(schedule.inputs, null, 2)}</code></pre>
			</details>
		{/if}
	</section>

	<footer>
		<button
			type="button"
			data-variant="secondary"
			onclick={onClose}
			use:autofocus
		>
			Close
		</button>
	</footer>
</dialog>

<style>
	dialog {
		max-width: 32rem;
		width: 100%;
	}
</style>
