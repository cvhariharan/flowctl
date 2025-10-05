<script lang="ts">
  import { currentUser } from '$lib/stores/auth';
  import { apiClient } from '$lib/apiClient';
  import { handleInlineError } from '$lib/utils/errorHandling';
  import { IconChevronDown } from '@tabler/icons-svelte';

  let userSettingsOpen = $state(false);

  const getUserInitials = (username: string): string => {
    return username.charAt(0).toUpperCase();
  };

  const logout = async () => {
    try {
      await apiClient.auth.logout();
      window.location.href = '/login';
    } catch (error) {
      handleInlineError(error, 'Unable to Log Out');
      // Force redirect even if logout fails
      window.location.href = '/login';
    }
  };

  // Handle outside clicks
  function handleOutsideClick(event: MouseEvent) {
    const target = event.target as HTMLElement;
    if (!target.closest('.user-dropdown-container')) {
      userSettingsOpen = false;
    }
  }
</script>

<svelte:window on:click={handleOutsideClick} />

<!-- User Menu -->
<div class="relative user-dropdown-container">
  <button
    type="button"
    onclick={() => userSettingsOpen = !userSettingsOpen}
    class="w-full flex items-center px-3 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-lg hover:bg-gray-50 transition-colors"
    aria-label="User menu toggle"
  >
    <div class="w-8 h-8 bg-primary-500 rounded-full flex items-center justify-center flex-shrink-0">
      <span class="text-white font-semibold text-sm">
        {$currentUser ? getUserInitials($currentUser.username) : 'U'}
      </span>
    </div>
    <div class="ml-3 flex-1 text-left">
      <div class="text-sm font-medium text-gray-900">{$currentUser?.username || 'Loading...'}</div>
      <div class="text-xs text-gray-500 capitalize">{$currentUser?.role || ''}</div>
    </div>
    <IconChevronDown
      class="text-gray-500 transition-transform flex-shrink-0 {userSettingsOpen ? 'rotate-180' : ''}"
      size={16}
    />
  </button>

  <!-- Dropdown Menu -->
  {#if userSettingsOpen}
    <div
      class="absolute bottom-full left-0 w-full mb-1 bg-white rounded-lg shadow-lg border border-gray-200"
      role="menu"
      aria-label="User menu"
    >
      <div class="py-1">
        <button
          type="button"
          onclick={logout}
          class="w-full text-left px-3 py-2 text-sm text-gray-700 hover:bg-gray-100 transition-colors"
          role="menuitem"
        >
          Logout
        </button>
      </div>
    </div>
  {/if}
</div>