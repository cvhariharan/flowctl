<script lang="ts">
  import IconX from '@tabler/icons-svelte/icons/x';
  import type { FlowProblem } from '$lib/types';

  let {
    problems,
    onSelect,
    onClose
  }: {
    problems: FlowProblem[];
    onSelect: (actionId: string) => void;
    onClose: () => void;
  } = $props();
</script>

<div class="strip">
  <div class="hstack justify-between mb-2">
    <h4 class="overline text-lighter">Problems</h4>
    <button type="button" class="ghost icon small" aria-label="Hide problems" onclick={onClose}>
      <IconX size={14} />
    </button>
  </div>
  <ul class="unstyled vstack gap-1 text-xs">
    {#each problems as problem, i (i)}
      <li class="hstack gap-2 nowrap items-start">
        <span class="marker" class:warning={problem.severity === 'warning'} aria-hidden="true">
          {problem.severity === 'warning' ? '!' : '×'}
        </span>
        <span>
          {problem.message}
          {#if problem.actionId}
            {@const actionId = problem.actionId}
            <button type="button" class="ghost small" onclick={() => onSelect(actionId)}>
              show
            </button>
          {/if}
        </span>
      </li>
    {/each}
  </ul>
</div>

<style>
  .strip {
    flex-shrink: 0;
    max-height: 11rem;
    overflow-y: auto;
    padding: var(--space-3) var(--space-4);
    background: var(--card);
    border-top: 1px solid var(--border);
  }
  h4 {
    margin: 0;
  }
  .marker {
    color: var(--danger);
  }
  .marker.warning {
    color: var(--warning);
  }
</style>
