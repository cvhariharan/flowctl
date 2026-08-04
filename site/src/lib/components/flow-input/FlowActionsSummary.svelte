<script lang="ts">
  import type { FlowAction } from '$lib/types';

  let {
    actions,
    title = 'Flow Actions'
  }: {
    actions: FlowAction[],
    title?: string
  } = $props();
</script>

{#if actions && actions.length > 0}
  <article class="card">
    <header class="card-list-header">
      <h3 class="text-sm font-medium">
        {title} ({actions.length})
      </h3>
    </header>
    <div class="vstack gap-1">
      {#each actions as action, index}
        <div class="action-item">
          <div class="action-number">
            {index + 1}
          </div>
          <div class="text-sm action-name">{action.name}</div>
          <span class="badge primary shrink-0">
            {action.executor}
          </span>
        </div>
      {/each}
    </div>
  </article>
{/if}

<style>
  .card-list-header {
    padding: var(--space-3) var(--space-4);
    border-bottom: 1px solid var(--border);
  }
  .action-item {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-3) var(--space-4);
    border-bottom: 1px solid var(--border);
  }
  .action-item:last-child {
    border-bottom: none;
  }
  .action-item:hover {
    background: var(--faint);
  }
  .action-name {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .action-number {
    flex-shrink: 0;
    width: 1.5rem;
    height: 1.5rem;
    background: var(--faint);
    color: var(--primary);
    border-radius: var(--radius-small);
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: var(--text-8);
    font-weight: var(--font-medium);
  }
</style>
