<script lang="ts">
  import { goto } from '$app/navigation';
  import type { ComponentType } from 'svelte';

  type BreadcrumbItem = {
    label: string;
    url?: string;
  };

  let {
    breadcrumbs = [],
    actions = [],
    children
  }: {
    breadcrumbs?: (string | BreadcrumbItem)[],
    actions?: Array<{ label: string, onClick: () => void, variant?: 'primary' | 'secondary' | 'danger' | 'ghost', icon?: ComponentType, tooltip?: string }>,
    children?: any
  } = $props();

  // Convert breadcrumbs to uniform format
  const normalizedBreadcrumbs = $derived(
    breadcrumbs.map(crumb =>
      typeof crumb === 'string' ? { label: crumb } : crumb
    )
  );

  const handleBreadcrumbClick = (crumb: BreadcrumbItem) => {
    if (crumb.url) {
      goto(crumb.url);
    }
  };
</script>

<header class="header-bar hstack justify-between">
  <div class="hstack gap-5">
    {#if normalizedBreadcrumbs.length > 0}
      <nav aria-label="Breadcrumb" class="hstack text-sm text-light">
        <ol class="hstack">
          {#each normalizedBreadcrumbs as crumb, index}
            <li class="hstack">
              {#if crumb.url && index < normalizedBreadcrumbs.length - 1}
                <button
                  onclick={() => handleBreadcrumbClick(crumb)}
                  class="breadcrumb-link"
                >
                  {crumb.label}
                </button>
              {:else}
                <span class={index === normalizedBreadcrumbs.length - 1 ? '' : 'text-light'} aria-current={index === normalizedBreadcrumbs.length - 1 ? 'page' : undefined}>{crumb.label}</span>
              {/if}
              {#if index < normalizedBreadcrumbs.length - 1}
                <span class="mx-2" aria-hidden="true">/</span>
              {/if}
            </li>
          {/each}
        </ol>
      </nav>
    {/if}
  </div>

  <div class="header-actions hstack gap-4 nowrap">
    {#if children}
      {@render children()}
    {/if}

    {#each actions as action}
      <button
        onclick={action.onClick}
        data-tooltip={action.tooltip || undefined}
        data-variant={action.variant === 'danger' ? 'danger' : action.variant === 'secondary' ? 'secondary' : action.variant === 'ghost' ? 'ghost' : undefined}
        class="hstack gap-2"
      >
        {#if action.icon}
          {@const Icon = action.icon}
          <Icon size={16} />
        {/if}
        {action.label}
      </button>
    {/each}
  </div>
</header>

<style>
  .header-bar {
    position: relative;
    z-index: 10;
    background: var(--card);
    border-bottom: 1px solid var(--border);
    padding: var(--space-3) var(--space-6);
    height: 3.75rem;
    align-items: center;
    align-content: center;
    flex-wrap: nowrap;
    overflow: visible;
  }

  .header-actions > button[data-tooltip]::after {
    inset-block-start: calc(100% + 10px);
    inset-block-end: auto;
    inset-inline-start: auto;
    inset-inline-end: 0;
    transform: translateY(-4px);
  }

  .header-actions > button[data-tooltip]::before {
    inset-block-start: calc(100% - 5px);
    inset-block-end: auto;
    border-top-color: transparent;
    border-bottom-color: var(--foreground);
    transform: translate(-50%) translateY(-4px);
  }

  .header-actions > button[data-tooltip]:is(:hover, :focus-visible)::after {
    transform: translateY(0);
  }

  .header-actions > button[data-tooltip]:is(:hover, :focus-visible)::before {
    transform: translate(-50%) translateY(0);
  }

  .breadcrumb-link {
    all: unset;
    cursor: pointer;
    color: var(--muted-foreground);
    transition: color 0.15s ease;
  }

  .breadcrumb-link:hover {
    color: var(--primary);
    text-decoration: underline;
    text-underline-offset: 0.2em;
  }
</style>
