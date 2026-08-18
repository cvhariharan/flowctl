<script lang="ts">
  import { apiClient } from '$lib/apiClient';
  import { handleInlineError } from '$lib/utils/errorHandling';
  import type { User, Group } from '$lib/types';
  import IconUsers from '@tabler/icons-svelte/icons/users';
  import IconUser from '@tabler/icons-svelte/icons/user';

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
  let loading = $state(false);
  let searchInput: HTMLInputElement;
  let triggerEl: HTMLButtonElement;
  let popoverEl: HTMLDivElement;

  async function loadSubjects() {
    loading = true;
    try {
      if (type === 'user') {
        const response = await apiClient.users.list({ filter: searchQuery, count_per_page: 20 });
        searchResults = response.users || [];
      } else {
        const response = await apiClient.groups.list({ filter: searchQuery, count_per_page: 20 });
        searchResults = response.groups || [];
      }
    } catch (error) {
      handleInlineError(error, type === 'user' ? 'Unable to Load Users' : 'Unable to Load Groups');
      searchResults = [];
      closeDropdown();
    } finally {
      loading = false;
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
    loadSubjects();
    setTimeout(() => searchInput?.focus(), 0);
  }

  function closeDropdown() {
    try { popoverEl?.hidePopover(); } catch {}
  }

  let searchTimer: number;
  function handleSearchInput() {
    clearTimeout(searchTimer);
    searchTimer = setTimeout(() => loadSubjects(), 300);
  }

  function selectSubject(subject: User | Group) {
    selectedSubject = subject;
    searchQuery = '';
    closeDropdown();
    searchResults = [];
  }

  function clearSelection() {
    selectedSubject = null;
    searchQuery = '';
    searchResults = [];
  }

  $effect(() => {
    clearSelection();
  });

  function handleOutsideClick(event: MouseEvent) {
    const target = event.target as HTMLElement;
    if (!target.closest('.user-group-selector') && !target.closest('[popover]')) {
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

<div class="user-group-selector vstack gap-2">
  <button
    type="button"
    bind:this={triggerEl}
    onclick={openDropdown}
    class="outline small w-100 hstack justify-between"
    {disabled}
  >
    <span class="text-lighter">{placeholder}</span>
  </button>

  <div bind:this={popoverEl} popover="manual" class="selector-popover">
    <div style="padding: var(--space-2); border-bottom: 1px solid var(--border)">
      <input
        type="search"
        bind:this={searchInput}
        bind:value={searchQuery}
        oninput={handleSearchInput}
        {placeholder}
        aria-busy={loading}
        autocomplete="off"
      />
    </div>
    <div style="max-height: 14rem; overflow-y: auto">
      {#if searchResults.length > 0}
        {#each searchResults as subject}
          <button type="button" class="dropdown-item" onclick={() => selectSubject(subject)}>
            <div class="icon-box">
              {#if type === 'user'}
                <IconUser size={16} />
              {:else}
                <IconUsers size={16} />
              {/if}
            </div>
            <div>
              <div class="font-medium">{'name' in subject ? subject.name : subject.username}</div>
              <div class="text-lighter text-xs">{subject.id}</div>
            </div>
          </button>
        {/each}
      {:else if !loading}
        <div class="dropdown-empty">
          {type === 'user' ? 'No users found' : 'No groups found'}
        </div>
      {/if}
    </div>
  </div>

  {#if selectedSubject}
    <div class="hstack justify-between" style="padding: 0.5rem; background: var(--faint); border-radius: 0.5rem; border: 1px solid var(--border)">
      <div class="hstack">
        <div class="icon-box">
          {#if type === 'user'}
            <IconUser size={16} />
          {:else}
            <IconUsers size={16} />
          {/if}
        </div>
        <div>
          <div class="font-medium text-sm">{'name' in selectedSubject ? selectedSubject.name : selectedSubject.username}</div>
          <div class="text-lighter text-xs">{selectedSubject.id}</div>
        </div>
      </div>
      <button type="button" class="ghost icon small" onclick={clearSelection} {disabled} aria-label="Clear selection">
        <svg style="width: 1rem; height: 1rem" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/>
        </svg>
      </button>
    </div>
  {/if}
</div>
