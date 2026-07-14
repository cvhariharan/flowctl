<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { apiClient } from '$lib/apiClient';
  import { handleInlineError } from '$lib/utils/errorHandling';
  import type { NodeResp } from '$lib/types';
  import { IconChevronDown, IconServer, IconTag, IconX } from '@tabler/icons-svelte';

  let {
    namespace,
    selectedNodes = $bindable([]),
    placeholder = 'Search nodes or use tag:name...',
    disabled = false,
    multiple = true,
    flowId = ''
  }: {
    namespace: string;
    selectedNodes: string[];
    placeholder?: string;
    disabled?: boolean;
    multiple?: boolean;
    flowId?: string;
  } = $props();

  const listNodes = (params: { count_per_page?: number; filter?: string; tags?: string[] }) =>
    flowId
      ? apiClient.nodes.listForFlow(namespace, flowId, params)
      : apiClient.nodes.list(namespace, params);

  let searchQuery = $state('');
  let searchResults = $state<NodeResp[]>([]);
  let loading = $state(false);
  let searchTimeout: ReturnType<typeof setTimeout> | null = null;
  let isTagSearch = $state(false);
  let currentTagQuery = $state('');
  let searchInput: HTMLInputElement;
  let triggerEl: HTMLButtonElement;
  let popoverEl: HTMLDivElement;

  const triggerLabel = $derived(
    selectedNodes.length > 0
      ? `${selectedNodes.length} selected`
      : placeholder
  );

  async function searchNodes(filter: string = '') {
    loading = true;
    isTagSearch = false;
    currentTagQuery = '';

    if (filter.startsWith('tag:')) {
      const tagName = filter.slice(4).trim();
      isTagSearch = true;
      currentTagQuery = tagName;

      if (tagName) {
        try {
          const response = await listNodes({ count_per_page: 100, tags: [tagName] });
          searchResults = response.nodes || [];
        } catch (error) {
          handleInlineError(error, 'Unable to Load Nodes');
          searchResults = [];
        } finally {
          loading = false;
        }
        return;
      } else {
        searchResults = [];
        loading = false;
        return;
      }
    }

    try {
      const response = await listNodes({ count_per_page: 100, filter });
      searchResults = response.nodes || [];
    } catch (error) {
      handleInlineError(error, 'Unable to Load Nodes');
      searchResults = [];
    } finally {
      loading = false;
    }
  }

  function isTagSelection(item: string): boolean {
    return item.startsWith('tag:');
  }

  function positionPopover() {
    if (!triggerEl || !popoverEl) return;
    const r = triggerEl.getBoundingClientRect();
    popoverEl.style.top = `${r.bottom + 4}px`;
    popoverEl.style.left = `${r.left}px`;
    popoverEl.style.width = `${r.width}px`;
  }

  function openDropdown() {
    if (disabled) return;
    try { popoverEl?.showPopover(); } catch {}
    positionPopover();
    searchQuery = '';
    searchNodes();
    setTimeout(() => searchInput?.focus(), 0);
  }

  function closeDropdown() {
    try { popoverEl?.hidePopover(); } catch {}
  }

  function handleInput() {
    if (searchTimeout) clearTimeout(searchTimeout);
    searchTimeout = setTimeout(() => searchNodes(searchQuery), 300);
  }

  function selectNode(node: NodeResp) {
    if (multiple) {
      if (!selectedNodes.includes(node.name)) {
        selectedNodes = [...selectedNodes, node.name];
      }
    } else {
      selectedNodes = [node.name];
      closeDropdown();
    }
    searchQuery = '';
  }

  function selectTag(tagName: string) {
    const tagValue = `tag:${tagName}`;
    if (multiple) {
      if (!selectedNodes.includes(tagValue)) {
        selectedNodes = [...selectedNodes, tagValue];
      }
    } else {
      selectedNodes = [tagValue];
      closeDropdown();
    }
    searchQuery = '';
  }

  function removeNode(nodeName: string) {
    selectedNodes = selectedNodes.filter(n => n !== nodeName);
  }

  function handleOutsideClick(event: MouseEvent) {
    const target = event.target as HTMLElement;
    if (!target.closest('.node-selector') && !target.closest('[popover]')) {
      closeDropdown();
    }
  }

  let scrollCleanup: (() => void) | null = null;

  onMount(() => {
    if (!disabled) searchNodes();

    const onScroll = () => {
      if (popoverEl?.matches(':popover-open')) closeDropdown();
    };
    document.addEventListener('scroll', onScroll, { capture: true, passive: true });
    scrollCleanup = () => document.removeEventListener('scroll', onScroll, { capture: true });
  });

  onDestroy(() => scrollCleanup?.());
</script>

<svelte:window on:click={handleOutsideClick} />

<div class="node-selector">
  <button
    type="button"
    bind:this={triggerEl}
    onclick={openDropdown}
    class="outline w-100 hstack justify-between"
    {disabled}
  >
    <span class="text-lighter">{triggerLabel}</span>
    {#if loading}
      <span aria-busy="true" data-spinner="small"></span>
    {:else}
      <IconChevronDown size={16} />
    {/if}
  </button>

  <div bind:this={popoverEl} popover="manual" class="selector-popover">
    <div style="padding: var(--space-2); border-bottom: 1px solid var(--border)">
      <input
        type="search"
        bind:this={searchInput}
        bind:value={searchQuery}
        oninput={handleInput}
        placeholder="Search nodes or tag:name..."
        autocomplete="off"
      />
    </div>

    <div style="max-height: 14rem; overflow-y: auto;">
      {#if isTagSearch && currentTagQuery && !selectedNodes.includes(`tag:${currentTagQuery}`)}
        <button type="button" class="dropdown-item" onclick={() => selectTag(currentTagQuery)}>
          <div class="icon-box success" style="margin-right: var(--space-2)">
            <IconTag size={16} />
          </div>
          <div>
            <div>Select tag: {currentTagQuery}</div>
            <div class="text-lighter text-xs">{searchResults.length} node{searchResults.length !== 1 ? 's' : ''} with this tag</div>
          </div>
        </button>
      {/if}

      {#if searchResults.length > 0}
        {#if isTagSearch}
          <div class="text-lighter text-xs" style="padding: 0.25rem 1rem; background: var(--faint); border-bottom: 1px solid var(--border);">
            Or select individual nodes:
          </div>
        {/if}
        {#each searchResults as node}
          {#if !selectedNodes.includes(node.name)}
            <button type="button" class="dropdown-item" onclick={() => selectNode(node)}>
              <div class="icon-box" style="margin-right: var(--space-2)">
                <IconServer size={16} />
              </div>
              <div>
                <div>{node.name}</div>
                <div class="text-lighter text-xs">{node.hostname}:{node.port}</div>
              </div>
            </button>
          {/if}
        {/each}
      {:else if !loading}
        <div class="text-lighter text-sm align-center" style="padding: var(--space-3) var(--space-4)">
          {searchQuery ? 'No nodes found' : 'No nodes available'}
        </div>
      {/if}
    </div>
  </div>

  {#if selectedNodes.length > 0}
    <div class="hstack mt-2" style="flex-wrap: wrap; gap: 0.25rem">
      {#each selectedNodes as nodeName (nodeName)}
        {#if isTagSelection(nodeName)}
          <span class="badge success hstack gap-1">
            <IconTag size={12} />
            {nodeName.slice(4)}
            {#if !disabled}
              <button type="button" onclick={() => removeNode(nodeName)} aria-label="Remove {nodeName}" style="all: unset; cursor: pointer; display: flex">
                <IconX size={12} />
              </button>
            {/if}
          </span>
        {:else}
          <span class="badge hstack gap-1">
            {nodeName}
            {#if !disabled}
              <button type="button" onclick={() => removeNode(nodeName)} aria-label="Remove {nodeName}" style="all: unset; cursor: pointer; display: flex">
                <IconX size={12} />
              </button>
            {/if}
          </span>
        {/if}
      {/each}
    </div>
  {/if}
</div>

