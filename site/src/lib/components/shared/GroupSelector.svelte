<script lang="ts">
  import { onMount } from 'svelte';
  import { apiClient } from '$lib/apiClient';
  import { handleInlineError } from '$lib/utils/errorHandling';
  import type { Group } from '$lib/types';
  import { IconChevronDown, IconUsersGroup, IconX } from '@tabler/icons-svelte';
  
  let {
    selectedGroups = $bindable([]),
    placeholder = 'Search groups...',
    disabled = false,
    multiple = true
  }: {
    selectedGroups: Group[];
    placeholder?: string;
    disabled?: boolean;
    multiple?: boolean;
  } = $props();

  let searchQuery = $state('');
  let searchResults = $state<Group[]>([]);
  let allGroups = $state<Group[]>([]);
  let showDropdown = $state(false);
  let loading = $state(false);
  let initialized = $state(false);

  async function loadAllGroups() {
    if (initialized) return;
    
    loading = true;
    try {
      const response = await apiClient.groups.list({
        count_per_page: 100 // Get more groups for selection
      });
      allGroups = response.groups || [];
      searchResults = allGroups;
      initialized = true;
    } catch (error) {
      handleInlineError(error, 'Unable to Load Groups');
      allGroups = [];
      searchResults = [];
    } finally {
      loading = false;
    }
  }

  function filterGroups() {
    if (!searchQuery.trim()) {
      searchResults = allGroups;
    } else {
      searchResults = allGroups.filter(group => 
        group.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
        (group.description && group.description.toLowerCase().includes(searchQuery.toLowerCase()))
      );
    }
  }

  async function handleInput() {
    filterGroups();
  }

  async function handleFocus() {
    filterGroups();
    showDropdown = searchResults.length > 0;
  }

  function selectGroup(group: Group) {
    if (multiple) {
      if (!selectedGroups.some(g => g.id === group.id)) {
        selectedGroups = [...selectedGroups, group];
      }
    } else {
      selectedGroups = [group];
    }
    searchQuery = '';
    showDropdown = false;
  }

  function removeGroup(groupId: string) {
    selectedGroups = selectedGroups.filter(g => g.id !== groupId);
  }


  // Load groups when component mounts
  onMount(() => {
    loadAllGroups();
  });

  // Close dropdown when clicking outside
  function handleOutsideClick(event: MouseEvent) {
    const target = event.target as HTMLElement;
    if (!target.closest('.group-selector')) {
      showDropdown = false;
    }
  }
</script>

<svelte:window on:click={handleOutsideClick} />

<div class="group-selector">
  <div class="relative">
    <input 
      type="text"
      bind:value={searchQuery}
      oninput={handleInput}
      onfocus={handleFocus}
      {placeholder}
      class="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-primary-500 pr-10"
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
      <IconChevronDown class="absolute right-3 top-1/2 transform -translate-y-1/2 text-gray-400" size={16} />
    {/if}
    
    <!-- Dropdown -->
    {#if showDropdown}
      <div class="absolute z-20 w-full mt-2 bg-white border border-gray-200 rounded-lg shadow-lg max-h-48 overflow-y-auto">
        {#if searchResults.length > 0}
          {#each searchResults as group}
            <!-- Skip already selected groups -->
            {#if !selectedGroups.some(g => g.id === group.id)}
              <div 
                class="px-4 py-2 hover:bg-gray-50 cursor-pointer border-b border-gray-100 last:border-b-0"
                onclick={() => selectGroup(group)}
                role="button"
                tabindex="0"
                onkeydown={(e) => e.key === 'Enter' && selectGroup(group)}
              >
                <div class="flex items-center">
                  <div class="w-8 h-8 rounded-lg flex items-center justify-center mr-3 bg-primary-50">
                    <IconUsersGroup class="text-primary-500" size={16} />
                  </div>
                  <div>
                    <div class="text-sm font-medium text-gray-900">{group.name}</div>
                    <div class="text-xs text-gray-500">{group.description || 'No description'}</div>
                  </div>
                </div>
              </div>
            {/if}
          {/each}
        {:else if initialized && !loading}
          <div class="px-4 py-2 text-sm text-gray-500 text-center">
            {searchQuery ? 'No groups found' : 'No groups available'}
          </div>
        {/if}
      </div>
    {/if}
  </div>
  
  <!-- Selected groups display -->
  {#if selectedGroups.length > 0}
    <div class="mt-2 flex flex-wrap gap-1">
      {#each selectedGroups as group (group.id)}
        <span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-primary-50 text-primary-900">
          {group.name}
          {#if !disabled}
            <button
              type="button"
              onclick={() => removeGroup(group.id)}
              class="ml-1 text-primary-500 hover:text-primary-900"
              aria-label="Remove {group.name}"
            >
              <IconX size={12} />
            </button>
          {/if}
        </span>
      {/each}
    </div>
  {/if}
</div>