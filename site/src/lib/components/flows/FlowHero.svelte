<script lang="ts">
  import IconLayoutGrid from '@tabler/icons-svelte/icons/layout-grid';
  import type { ComponentType } from 'svelte';

  let {
    name,
    description,
    IconComponent = IconLayoutGrid
  }: {
    name: string,
    description?: string,
    IconComponent?: ComponentType
  } = $props();
</script>

<div class="hero-section">
  <div class="hero-inner hstack gap-4 items-center">
    <div class="icon-box shrink-0" style="width:2.5rem;height:2.5rem">
      <IconComponent size={20} />
    </div>
    <div class="flex-1 min-w-0">
      <h1 class="hero-title">{name}</h1>
      {#if description}
        <p class="text-light hero-description">{description}</p>
      {/if}
    </div>
  </div>
</div>

<style>
  .hero-section {
    background: var(--card);
    padding: var(--space-5) var(--space-6) var(--space-4);
  }
  .hero-inner {
    max-width: 72rem;
    margin: 0 auto;
  }
  .hero-title {
    font-size: var(--text-4);
    font-weight: var(--font-semibold);
    margin: 0;
    line-height: 1.3;
  }
  .hero-description {
    font-size: var(--text-7);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    margin: var(--space-1) 0 0;
  }
</style>
