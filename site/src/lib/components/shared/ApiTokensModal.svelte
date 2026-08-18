<script lang="ts">
  import { apiClient } from '$lib/apiClient';
  import { handleInlineError } from '$lib/utils/errorHandling';
  import { autofocus } from '$lib/utils/autofocus';
  import DeleteModal from './DeleteModal.svelte';
  import IconCopy from '@tabler/icons-svelte/icons/copy';
  import IconCheck from '@tabler/icons-svelte/icons/check';
  import IconTrash from '@tabler/icons-svelte/icons/trash';
  import type { ApiToken, ApiTokenCreated } from '$lib/types';

  let { onClose }: { onClose: () => void } = $props();

  let dialogEl: HTMLDialogElement;
  let tokens = $state<ApiToken[]>([]);
  let loading = $state(true);
  let creating = $state(false);
  let label = $state('');
  let newToken = $state<ApiTokenCreated | null>(null);
  let copied = $state(false);
  let tokenToRevoke = $state<ApiToken | null>(null);

  async function loadTokens() {
    loading = true;
    try {
      tokens = (await apiClient.apiTokens.list()) ?? [];
    } catch (err) {
      handleInlineError(err, 'Unable to load API tokens');
    } finally {
      loading = false;
    }
  }

  async function handleCreate(event: Event) {
    event.preventDefault();
    if (!label.trim() || creating) return;
    creating = true;
    try {
      newToken = await apiClient.apiTokens.create(label.trim());
      label = '';
      await loadTokens();
    } catch (err) {
      handleInlineError(err, 'Unable to create API token');
    } finally {
      creating = false;
    }
  }

  async function copyToken() {
    if (!newToken) return;
    try {
      await navigator.clipboard.writeText(newToken.token);
      copied = true;
      setTimeout(() => (copied = false), 2000);
    } catch (err) {
      handleInlineError(err, 'Unable to copy token');
    }
  }

  async function confirmRevoke() {
    if (!tokenToRevoke) return;
    await apiClient.apiTokens.revoke(tokenToRevoke.uuid);
    tokenToRevoke = null;
    await loadTokens();
  }

  function formatDate(value?: string): string {
    if (!value) return '—';
    const d = new Date(value);
    if (Number.isNaN(d.getTime())) return value;
    return d.toLocaleString();
  }

  function handleClose() {
    dialogEl?.close();
    onClose();
  }

  $effect(() => {
    if (dialogEl) {
      dialogEl.showModal();
      loadTokens();
    }
  });
</script>

<dialog bind:this={dialogEl} onclose={onClose}>
  <header>
    <h3>API Tokens</h3>
    <p class="text-light">
      Authenticate API clients. Tokens carry the same permissions as your user.
    </p>
  </header>

  <section>
    {#if newToken}
      <div role="alert" data-variant="warning" class="mb-4">
        <strong>Copy this token, it will not be shown again.</strong>
        <fieldset class="group mt-2">
          <input type="text" value={newToken.token} readonly />
          <button type="button" onclick={copyToken} data-variant="secondary">
            {#if copied}
              <IconCheck size={14} /> Copied
            {:else}
              <IconCopy size={14} /> Copy
            {/if}
          </button>
        </fieldset>
      </div>
    {:else}
      <form onsubmit={handleCreate} class="mb-4">
        <fieldset class="group">
          <input
            type="text"
            placeholder="New token label (e.g. ci-pipeline)"
            bind:value={label}
            maxlength="150"
            required
            disabled={creating}
            use:autofocus
          />
          <button type="submit" disabled={creating || !label.trim()}>
            <span class="hstack gap-2 justify-center" aria-busy={creating} data-spinner="small">
              Create token
            </span>
          </button>
        </fieldset>
      </form>
    {/if}

    {#if loading}
      <div class="text-center text-light" aria-busy="true" data-spinner="small">Loading tokens…</div>
    {:else if tokens.length === 0}
      <p class="text-light text-center">No API tokens yet.</p>
    {:else}
      <div class="token-list">
        <table>
          <thead>
            <tr>
              <th>Label</th>
              <th>Created</th>
              <th>Last used</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {#each tokens as t (t.uuid)}
              <tr>
                <td>{t.label}</td>
                <td>{formatDate(t.created_at)}</td>
                <td>{formatDate(t.last_used_at)}</td>
                <td class="text-right">
                  <button
                    type="button"
                    class="ghost icon small"
                    onclick={() => (tokenToRevoke = t)}
                    aria-label="Revoke token {t.label}"
                    data-tooltip="Revoke"
                  >
                    <IconTrash size={16} />
                  </button>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  </section>

  <footer>
    <button type="button" onclick={handleClose} data-variant="secondary">Close</button>
  </footer>
</dialog>

{#if tokenToRevoke}
  <DeleteModal
    title="Revoke API token"
    itemName={tokenToRevoke.label}
    description="Any client using this token will lose access immediately."
    onConfirm={confirmRevoke}
    onClose={() => (tokenToRevoke = null)}
  />
{/if}

<style>
  dialog {
    width: 100%;
    max-width: 44rem;
    max-height: 85vh;
  }

  .token-list {
    max-height: 18rem;
    overflow-y: auto;
    border: 1px solid var(--border);
    border-radius: var(--radius-medium);
  }

  .token-list thead th {
    position: sticky;
    top: 0;
    background: var(--card);
  }
</style>
