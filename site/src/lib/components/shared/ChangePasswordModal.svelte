<script lang="ts">
  import { apiClient } from '$lib/apiClient';
  import { handleInlineError } from '$lib/utils/errorHandling';
  import { autofocus } from '$lib/utils/autofocus';

  let { onClose }: { onClose: () => void } = $props();

  let dialogEl: HTMLDialogElement;
  let oldPassword = $state('');
  let newPassword = $state('');
  let confirmPassword = $state('');
  let saving = $state(false);
  let success = $state(false);
  let error = $state('');

  async function handleSubmit(event: Event) {
    event.preventDefault();
    error = '';

    if (newPassword !== confirmPassword) {
      error = 'New passwords do not match';
      return;
    }

    if (newPassword.length < 8) {
      error = 'New password must be at least 8 characters';
      return;
    }

    saving = true;
    try {
      await apiClient.users.changePassword(oldPassword, newPassword);
      success = true;
      oldPassword = '';
      newPassword = '';
      confirmPassword = '';
    } catch (err) {
      handleInlineError(err, 'Unable to change password');
      error = err instanceof Error ? err.message : 'Failed to change password';
    } finally {
      saving = false;
    }
  }

  function handleClose() {
    dialogEl?.close();
    onClose();
  }

  $effect(() => {
    if (dialogEl) {
      dialogEl.showModal();
    }
  });
</script>

<dialog bind:this={dialogEl} onclose={onClose}>
  <header>
    <h3>Change Password</h3>
    <p class="text-light">
      Update your account password.
    </p>
  </header>

  <section>
    {#if success}
      <div role="alert" data-variant="success" class="mb-4">
        <strong>Password changed successfully.</strong>
      </div>
    {:else}
      <form onsubmit={handleSubmit}>
        {#if error}
          <div role="alert" data-variant="danger" class="mb-4">
            {error}
          </div>
        {/if}

        <fieldset>
          <label for="old-password">Current Password</label>
          <input
            type="password"
            id="old-password"
            bind:value={oldPassword}
            required
            disabled={saving}
            use:autofocus
          />
        </fieldset>

        <fieldset>
          <label for="new-password">New Password</label>
          <input
            type="password"
            id="new-password"
            bind:value={newPassword}
            required
            minlength="8"
            disabled={saving}
          />
        </fieldset>

        <fieldset>
          <label for="confirm-password">Confirm New Password</label>
          <input
            type="password"
            id="confirm-password"
            bind:value={confirmPassword}
            required
            minlength="8"
            disabled={saving}
          />
        </fieldset>

        <footer>
          <button type="button" onclick={handleClose} data-variant="secondary" disabled={saving}>
            Cancel
          </button>
          <button type="submit" disabled={saving || !oldPassword || !newPassword || !confirmPassword}>
            <span class="hstack gap-2 justify-center" aria-busy={saving} data-spinner="small">
              Change Password
            </span>
          </button>
        </footer>
      </form>
    {/if}
  </section>

  {#if success}
    <footer>
      <button type="button" onclick={handleClose} data-variant="secondary">Close</button>
    </footer>
  {/if}
</dialog>

<style>
  dialog {
    width: 100%;
    max-width: 28rem;
  }

  form {
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }

  fieldset {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }
</style>
