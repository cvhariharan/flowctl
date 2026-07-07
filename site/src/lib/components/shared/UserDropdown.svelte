<script lang="ts">
  import { currentUser } from '$lib/stores/auth';
  import { apiClient } from '$lib/apiClient';
  import { handleInlineError } from '$lib/utils/errorHandling';
  import { clearPermissionCache } from '$lib/utils/permissions';
  import ApiTokensModal from './ApiTokensModal.svelte';
  import ChangePasswordModal from './ChangePasswordModal.svelte';

  let showApiTokens = $state(false);
  let showChangePassword = $state(false);

  const getUserInitials = (username: string): string => {
    return username.charAt(0).toUpperCase();
  };

  const logout = async () => {
    clearPermissionCache();
    try {
      await apiClient.auth.logout();
      window.location.href = '/login';
    } catch (error) {
      handleInlineError(error, 'Unable to Log Out');
      window.location.href = '/login';
    }
  };

  const openCredentials = () => {
    showApiTokens = true;
  };

  const openChangePassword = () => {
    showChangePassword = true;
  };
</script>

<ot-dropdown style="display: block" class="mt-2">
  <button popovertarget="user-menu" class="ghost w-100 hstack gap-2" style="justify-content: flex-start">
    <figure data-variant="avatar" class="small" aria-label={$currentUser?.name || 'User'}>
      <abbr title={$currentUser?.name || 'User'}>{$currentUser ? getUserInitials($currentUser.name) : 'U'}</abbr>
    </figure>
    <div style="text-align: left; flex: 1; min-width: 0;">
      <div class="text-sm font-medium">{$currentUser?.name || 'Loading...'}</div>
      <div class="text-lighter text-xs" style="text-transform: capitalize">{$currentUser?.role || ''}</div>
    </div>
  </button>
  <div popover id="user-menu">
    <button role="menuitem" onclick={openCredentials}>Credentials</button>
    <button role="menuitem" onclick={openChangePassword}>Change Password</button>
    <button role="menuitem" onclick={logout}>Logout</button>
  </div>
</ot-dropdown>

{#if showApiTokens}
  <ApiTokensModal onClose={() => (showApiTokens = false)} />
{/if}

{#if showChangePassword}
  <ChangePasswordModal onClose={() => (showChangePassword = false)} />
{/if}

<style>
  figure[data-variant="avatar"] {
    background: var(--primary);
    color: white;
  }
</style>
