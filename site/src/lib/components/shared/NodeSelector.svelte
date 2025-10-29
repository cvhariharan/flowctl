<script lang="ts">
  import { onMount } from 'svelte';
  import { apiClient } from '$lib/apiClient';
  import { handleInlineError } from '$lib/utils/errorHandling';
  import type { NodeResp } from '$lib/types';
  import { IconChevronDown, IconServer, IconX } from '@tabler/icons-svelte';

  let {
    namespace,
    selectedNodes = $bindable([]),
    placeholder = 'Search nodes...',
    disabled = false,
    multiple = true
  }: {
    namespace: string;
    selectedNodes: string[];
    placeholder?: string;
    disabled?: boolean;
    multiple?: boolean;
  } = $props();

  let searchQuery = $state('');
  let searchResults = $state<NodeResp[]>([]);
  let showDropdown = $state(false);
  let loading = $state(false);
  let searchTimeout: ReturnType<typeof setTimeout> | null = null;

  async function searchNodes(filter: string = '') {
    loading = true;
    try {
      const response = await apiClient.nodes.list(namespace, {
        count_per_page: 100,
        filter: filter
      });
      searchResults = response.nodes || [];
    } catch (error) {
      handleInlineError(error, 'Unable to Load Nodes');
      searchResults = [];
    } finally {
      loading = false;
    }
  }

  async function handleInput() {
    // Debounce search
    if (searchTimeout) {
      clearTimeout(searchTimeout);
    }

    searchTimeout = setTimeout(() => {
      searchNodes(searchQuery);
    }, 300);

    showDropdown = true;
  }

  async function handleFocus() {
    await searchNodes(searchQuery);
    showDropdown = true;
  }

  function selectNode(node: NodeResp) {
    if (multiple) {
      if (!selectedNodes.includes(node.name)) {
        selectedNodes = [...selectedNodes, node.name];
      }
    } else {
      selectedNodes = [node.name];
    }
    searchQuery = '';
    showDropdown = false;
  }

  function removeNode(nodeName: string) {
    selectedNodes = selectedNodes.filter(n => n !== nodeName);
  }


  // Load nodes when component mounts
  onMount(() => {
    searchNodes();
  });

  // Close dropdown when clicking outside
  function handleOutsideClick(event: MouseEvent) {
    const target = event.target as HTMLElement;
    if (!target.closest('.node-selector')) {
      showDropdown = false;
    }
  }
</script>

<svelte:window on:click={handleOutsideClick} />

<div class="node-selector">
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
          {#each searchResults as node}
            <!-- Skip already selected nodes -->
            {#if !selectedNodes.includes(node.name)}
              <div
                class="px-4 py-2 hover:bg-gray-50 cursor-pointer border-b border-gray-100 last:border-b-0"
                onclick={() => selectNode(node)}
                role="button"
                tabindex="0"
                onkeydown={(e) => e.key === 'Enter' && selectNode(node)}
              >
                <div class="flex items-center">
                  <div class="w-8 h-8 rounded-lg flex items-center justify-center mr-3 bg-primary-50">
                    <IconServer class="text-primary-500" size={16} />
                  </div>
                  <div>
                    <div class="text-sm font-medium text-gray-900">{node.name}</div>
                    <div class="text-xs text-gray-500">{node.hostname}:{node.port}</div>
                  </div>
                </div>
              </div>
            {/if}
          {/each}
        {:else if !loading}
          <div class="px-4 py-2 text-sm text-gray-500 text-center">
            {searchQuery ? 'No nodes found' : 'No nodes available'}
          </div>
        {/if}
      </div>
    {/if}
  </div>

  <!-- Selected nodes display -->
  {#if selectedNodes.length > 0}
    <div class="mt-2 flex flex-wrap gap-1">
      {#each selectedNodes as nodeName (nodeName)}
        <span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-primary-50 text-primary-900">
          {nodeName}
          {#if !disabled}
            <button
              type="button"
              onclick={() => removeNode(nodeName)}
              class="ml-1 text-primary-500 hover:text-primary-900 cursor-pointer"
              aria-label="Remove {nodeName}"
            >
              <IconX size={12} />
            </button>
          {/if}
        </span>
      {/each}
    </div>
  {/if}
</div>
