<script lang="ts">
  import { autofocus } from '$lib/utils/autofocus';
  import IconRefresh from '@tabler/icons-svelte/icons/refresh';
  import IconCircleCheck from '@tabler/icons-svelte/icons/circle-check';

  type Action = {
    id: string;
    name: string;
    approval?: boolean;
  };

  let {
    target,
    affectedActions,
    onConfirm,
    onClose,
  }: {
    target: Action;
    affectedActions: Action[];
    onConfirm: () => Promise<void>;
    onClose: () => void;
  } = $props();

  let rerunning = $state(false);
  let dialogEl: HTMLDialogElement;
  const approvalWillBeRequested = $derived(affectedActions.some((action) => action.approval));

  async function confirm() {
    rerunning = true;
    try {
      await onConfirm();
    } finally {
      rerunning = false;
    }
  }

  function close() {
    if (!rerunning) dialogEl?.close();
  }

  $effect(() => {
    dialogEl?.showModal();
  });
</script>

<dialog bind:this={dialogEl} onclose={onClose}>
  <header>
    <div class="hstack gap-4">
      <div class="icon-box" style="width:3rem;height:3rem">
        <IconRefresh size={24} />
      </div>
      <div>
        <h3>Re-run from {target.name}</h3>
        <p class="text-sm text-light">This will run the selected action and everything downstream.</p>
      </div>
    </div>
  </header>

  <section>
    <p>The following {affectedActions.length === 1 ? 'action' : `${affectedActions.length} actions`} will run again:</p>
    <ul>
      {#each affectedActions as action (action.id)}
        <li>{action.name}</li>
      {/each}
    </ul>
    {#if approvalWillBeRequested}
      <p class="approval-note">
        <IconCircleCheck size={17} />
        Approval will be requested again for approval-gated actions.
      </p>
    {/if}
  </section>

  <footer>
    <button type="button" onclick={close} disabled={rerunning} data-variant="secondary" use:autofocus>
      Cancel
    </button>
    <button type="button" onclick={confirm} disabled={rerunning} data-variant="primary">
      {#if rerunning}<span aria-busy="true"></span>{/if}
      Re-run actions
    </button>
  </footer>
</dialog>

<style>
  dialog { width: min(32rem, calc(100vw - 2rem)); }
  ul {
    max-height: 14rem;
    overflow-y: auto;
    margin-bottom: 0;
  }
  .approval-note {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    color: var(--warning);
  }
</style>
