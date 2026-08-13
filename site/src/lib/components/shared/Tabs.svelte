<script lang="ts">
  import { createEventDispatcher } from 'svelte';

  type Tab = {
    id: string;
    label: string;
    badge?: string | number;
    disabled?: boolean;
  };

  type Props = {
    tabs: Tab[];
    activeTab: string;
    variant?: 'default' | 'pills';
    size?: 'sm' | 'md' | 'lg';
  };

  let {
    tabs,
    activeTab = $bindable(),
    variant = 'default',
    size = 'md'
  }: Props = $props();

  const dispatch = createEventDispatcher<{
    change: { tabId: string; tab: Tab };
  }>();

  const handleTabClick = (tab: Tab) => {
    if (tab.disabled) return;

    activeTab = tab.id;
    dispatch('change', { tabId: tab.id, tab });
  };
</script>

<div role="tablist" class={variant === 'pills' ? 'pills' : ''}>
  {#each tabs as tab}
    {@const isActive = activeTab === tab.id}
    <button
      role="tab"
      aria-selected={isActive}
      type="button"
      onclick={() => handleTabClick(tab)}
      disabled={tab.disabled}
    >
      {tab.label}
      {#if tab.badge != null}
        <span class="badge primary">{tab.badge}</span>
      {/if}
    </button>
  {/each}
</div>

<style>
  [role="tab"] .badge {
    padding: 0 var(--space-2);
    line-height: 1.5;
  }
</style>
