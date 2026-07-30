<script lang="ts">
  import type { ComponentType } from 'svelte';

  let {
    title,
    subtitle,
    actions = [],
    children
  }: {
    title: string,
    subtitle?: string,
    actions?: Array<{
      label: string,
      onClick: () => void,
      variant?: 'primary' | 'secondary',
      icon?: string,
      IconComponent?: ComponentType,
      iconSize?: number
    }>,
    children?: any
  } = $props();
</script>

<header class="hstack justify-between mb-6">
  <div>
    <h2>{title}</h2>
    {#if subtitle}
      <p class="text-light">{subtitle}</p>
    {/if}
  </div>

  {#if children || actions.length > 0}
    <div class="hstack gap-3">
      {#if children}
        {@render children()}
      {/if}
      {#each actions as action}
        <button
          onclick={action.onClick}
          data-variant={action.variant === 'secondary' ? 'secondary' : undefined}
          aria-label={action.label}
          class="hstack gap-2"
        >
          {#if action.IconComponent}
            <action.IconComponent size={action.iconSize || 16} aria-hidden="true" />
          {:else if action.icon}
            {@html action.icon}
          {/if}
          {action.label}
        </button>
      {/each}
    </div>
  {/if}
</header>
