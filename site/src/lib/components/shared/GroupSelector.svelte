<script lang="ts">
  import { apiClient } from '$lib/apiClient';
  import { handleInlineError } from '$lib/utils/errorHandling';
  import type { Group } from '$lib/types';
  import { IconChevronDown, IconUsersGroup, IconX } from '@tabler/icons-svelte';

  let {
    selectedGroups = $bindable([]),
    placeholder = 'Select groups...',
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
  let loading = $state(false);
  let initialized = $state(false);
  let searchInput: HTMLInputElement;
  let triggerEl: HTMLButtonElement;
  let popoverEl: HTMLDivElement;

  const triggerLabel = $derived(
    selectedGroups.length > 0
      ? `${selectedGroups.length} group${selectedGroups.length > 1 ? 's' : ''} selected`
      : placeholder
  );

  const availableResults = $derived(
    searchResults.filter(g => !selectedGroups.some(s => s.id === g.id))
  );

  async function loadAllGroups() {
    if (initialized) return;
    loading = true;
    try {
      const response = await apiClient.groups.list({ count_per_page: 100 });
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
    loadAllGroups();
    filterGroups();
    setTimeout(() => searchInput?.focus(), 0);
  }

  function closeDropdown() {
    try { popoverEl?.hidePopover(); } catch {}
  }

  function selectGroup(group: Group) {
    if (multiple) {
      if (!selectedGroups.some(g => g.id === group.id)) {
        selectedGroups = [...selectedGroups, group];
      }
    } else {
      selectedGroups = [group];
      closeDropdown();
    }
    searchQuery = '';
  }

  function removeGroup(groupId: string) {
    selectedGroups = selectedGroups.filter(g => g.id !== groupId);
  }

  function handleOutsideClick(event: MouseEvent) {
    const target = event.target as HTMLElement;
    if (!target.closest('.group-selector') && !target.closest('[popover]')) {
      closeDropdown();
    }
  }

  import { onMount, onDestroy } from 'svelte';

  let scrollCleanup: (() => void) | null = null;

  onMount(() => {
    const onScroll = () => {
      if (popoverEl?.matches(':popover-open')) closeDropdown();
    };
    document.addEventListener('scroll', onScroll, { capture: true, passive: true });
    scrollCleanup = () => document.removeEventListener('scroll', onScroll, { capture: true });
  });

  onDestroy(() => scrollCleanup?.());
</script>

<svelte:window on:click={handleOutsideClick} />

<div class="group-selector">
  <button
    type="button"
    bind:this={triggerEl}
    onclick={openDropdown}
    class="outline w-100 hstack justify-between"
    {disabled}
    aria-busy={loading}
  >
    <span>{triggerLabel}</span>
    <IconChevronDown size={14} />
  </button>

  <div bind:this={popoverEl} popover="manual" class="selector-popover">
    <div style="padding: var(--space-2); border-bottom: 1px solid var(--border)">
      <input
        type="search"
        bind:this={searchInput}
        bind:value={searchQuery}
        oninput={filterGroups}
        placeholder="Search groups..."
        autocomplete="off"
      />
    </div>
    <div style="max-height: 14rem; overflow-y: auto">
      {#each availableResults as group (group.id)}
        <button type="button" class="dropdown-item" onclick={() => selectGroup(group)}>
          <div class="icon-box">
            <IconUsersGroup size={16} />
          </div>
          <div>
            <div class="font-medium">{group.name}</div>
            <div class="text-lighter text-xs">{group.description || 'No description'}</div>
          </div>
        </button>
      {/each}
      {#if availableResults.length === 0 && initialized && !loading}
        <div class="dropdown-empty">
          {searchQuery ? 'No groups found' : 'No groups available'}
        </div>
      {/if}
      {#if loading}
        <div class="dropdown-empty">Loading...</div>
      {/if}
    </div>
  </div>

  {#if selectedGroups.length > 0}
    <div class="hstack mt-2" style="flex-wrap: wrap; gap: 0.25rem">
      {#each selectedGroups as group (group.id)}
        <span class="badge primary hstack gap-1">
          {group.name}
          {#if !disabled}
            <button type="button" onclick={() => removeGroup(group.id)} aria-label="Remove {group.name}" style="all: unset; cursor: pointer; display: flex">
              <IconX size={12} />
            </button>
          {/if}
        </span>
      {/each}
    </div>
  {/if}
</div>
