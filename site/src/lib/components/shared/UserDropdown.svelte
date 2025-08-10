<script lang="ts">
  import { currentUser } from '$lib/stores/auth';
  import { apiClient } from '$lib/apiClient';
  import { handleInlineError } from '$lib/utils/errorHandling';

  let userSettingsOpen = $state(false);

  const getInitials = (name: string): string => {
    return name
      .split(' ')
      .map(n => n[0])
      .join('')
      .substring(0, 2)
      .toUpperCase();
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
</script>

<!-- User Menu -->
<div class="relative">
  <div 
    class="flex items-center space-x-3 cursor-pointer" 
    onclick={() => userSettingsOpen = !userSettingsOpen}
  >
    <div class="w-8 h-8 bg-blue-500 rounded-full flex items-center justify-center">
      <span class="text-white text-sm font-medium">
        {$currentUser ? getInitials($currentUser.name) : 'U'}
      </span>
    </div>
    <span class="text-gray-700 text-sm font-medium">
      {$currentUser ? $currentUser.name : 'Loading...'}
    </span>
    <svg 
      class="w-4 h-4 text-gray-500 transition-transform" 
      class:rotate-180={userSettingsOpen}
      fill="none" 
      stroke="currentColor" 
      viewBox="0 0 24 24"
    >
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7"></path>
    </svg>
  </div>
  
  <!-- Dropdown Menu -->
  {#if userSettingsOpen}
    <div 
      class="absolute right-0 mt-2 w-48 bg-white rounded-md shadow-lg ring-1 ring-black ring-opacity-5 z-50"
      role="menu"
    >
      <div class="py-1">
        <div class="px-4 py-2 text-sm text-gray-700 border-b border-gray-100">
          <div class="font-medium">{$currentUser?.name || ''}</div>
          <div class="text-gray-500">{$currentUser?.username || ''}</div>
          <div class="text-xs text-gray-400">{$currentUser?.role || ''}</div>
        </div>
        <a href="#" class="block px-4 py-2 text-sm text-gray-700 hover:bg-gray-100">Profile</a>
        <a href="#" class="block px-4 py-2 text-sm text-gray-700 hover:bg-gray-100">Settings</a>
        <div class="border-t border-gray-100"></div>
        <button 
          onclick={logout}
          class="block w-full text-left px-4 py-2 text-sm text-gray-700 hover:bg-gray-100"
        >
          Sign out
        </button>
      </div>
    </div>
  {/if}
</div>