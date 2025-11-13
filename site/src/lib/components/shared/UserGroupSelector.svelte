<script lang="ts">
  import { apiClient } from '$lib/apiClient';
  import { handleInlineError } from '$lib/utils/errorHandling';
  import type { User, Group } from '$lib/types';
  import { IconUsers, IconUser } from '@tabler/icons-svelte';
  
  let {
    type = $bindable('user'),
    selectedSubject = $bindable(null),
    placeholder = 'Search...',
    disabled = false
  }: {
    type: 'user' | 'group';
    selectedSubject: User | Group | null;
    placeholder?: string;
    disabled?: boolean;
  } = $props();

  let searchQuery = $state('');
  let searchResults = $state<(User | Group)[]>([]);
  let showDropdown = $state(false);
  let loading = $state(false);

  async function searchSubjects() {
    if (!searchQuery.trim()) {
      searchResults = [];
      showDropdown = false;
      return;
    }

    loading = true;
    try {
      if (type === 'user') {
        const response = await apiClient.users.list({
          filter: searchQuery,
          count_per_page: 10
        });
        searchResults = response.users || [];
      } else {
        const response = await apiClient.groups.list({
          filter: searchQuery,
          count_per_page: 10
        });
        searchResults = response.groups || [];
      }
      showDropdown = searchResults.length > 0;
    } catch (error) {
      handleInlineError(error, type === 'user' ? 'Unable to Search Users' : 'Unable to Search Groups');
      searchResults = [];
      showDropdown = false;
    } finally {
      loading = false;
    }
  }

  function selectSubject(subject: User | Group) {
    selectedSubject = subject;
    searchQuery = 'name' in subject ? subject.name : subject.username;
    showDropdown = false;
  }

  function clearSelection() {
    selectedSubject = null;
    searchQuery = '';
    searchResults = [];
    showDropdown = false;
  }

  // Reset when type changes
  $effect(() => {
    clearSelection();
  });

  // Close dropdown when clicking outside
  function handleOutsideClick(event: MouseEvent) {
    const target = event.target as HTMLElement;
    if (!target.closest('.user-group-selector')) {
      showDropdown = false;
    }
  }
</script>

<svelte:window on:click={handleOutsideClick} />

<div class="user-group-selector">
  <div class="relative">
    <input
      type="text"
      bind:value={searchQuery}
      on:input={searchSubjects}
      on:focus={() => showDropdown = searchResults.length > 0}
      {placeholder}
      class="bg-gray-50 border border-gray-300 text-gray-900 text-sm rounded-lg focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent block w-full p-2.5 pr-10"
      autocomplete="off"
      {disabled}
    />
    
    {#if loading}
      <div class="absolute right-3 top-1/2 transform -translate-y-1/2">
        <svg class="animate-spin h-4 w-4 text-gray-400" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
          <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
        </svg>
      </div>
    {:else}
      <svg class="w-5 h-5 absolute right-3 top-1/2 transform -translate-y-1/2 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"/>
      </svg>
    {/if}
    
    <!-- Dropdown -->
    {#if showDropdown && searchResults.length > 0}
      <div class="absolute z-10 w-full mt-1 bg-white border border-gray-300 rounded-lg shadow-lg max-h-48 overflow-y-auto">
        {#each searchResults as subject}
          <div 
            class="px-4 py-2 hover:bg-gray-50 cursor-pointer border-b border-gray-100 last:border-b-0"
            on:click={() => selectSubject(subject)}
            role="button"
            tabindex="0"
            on:keydown={(e) => e.key === 'Enter' && selectSubject(subject)}
          >
            <div class="flex items-center">
              <div class="w-8 h-8 rounded-lg flex items-center justify-center mr-3 bg-primary-50">
                {#if type === 'user'}
                  <IconUser class="w-4 h-4 text-primary-600" />
                {:else}
                  <IconUsers class="w-4 h-4 text-primary-600" />
                {/if}
              </div>
              <div>
                <div class="text-sm font-medium text-gray-900">{'name' in subject ? subject.name : subject.username}</div>
                <div class="text-xs text-gray-500">{subject.id}</div>
              </div>
            </div>
          </div>
        {/each}
      </div>
    {/if}
  </div>
  
  <!-- Selected subject display -->
  {#if selectedSubject}
    <div class="mt-2 p-2 bg-gray-50 rounded-lg border">
      <div class="flex items-center justify-between">
        <div class="flex items-center">
          <div class="w-8 h-8 rounded-lg flex items-center justify-center mr-3 bg-primary-50">
            {#if type === 'user'}
              <IconUser class="w-4 h-4 text-primary-600" />
            {:else}
              <IconUsers class="w-4 h-4 text-primary-600" />
            {/if}
          </div>
          <div>
            <div class="text-sm font-medium text-gray-900">{'name' in selectedSubject ? selectedSubject.name : selectedSubject.username}</div>
            <div class="text-xs text-gray-500">{selectedSubject.id}</div>
          </div>
        </div>
        <button type="button" on:click={clearSelection} class="text-gray-400 hover:text-gray-600" {disabled}>
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/>
          </svg>
        </button>
      </div>
    </div>
  {/if}
</div>