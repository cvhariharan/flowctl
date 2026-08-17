<script lang="ts">
  import type { FlowAction, ExecutionMode } from '$lib/types';
  import { actionStages } from '$lib/utils/dag';

  let {
    actions,
    executionMode = 'sequential',
    title = 'Flow Actions'
  }: {
    actions: FlowAction[],
    executionMode?: ExecutionMode,
    title?: string
  } = $props();

  const isDAG = $derived(executionMode === 'dag');
  const stages = $derived(actionStages(actions ?? [], isDAG ? 'dag' : 'sequential'));
  // needs holds action ids; the list reads better with the names the ids point at.
  const nameById = $derived(new Map((actions ?? []).map((action) => [action.id, action.name])));

  const dependencyNames = (needs: string[]) =>
    needs.map((dep) => nameById.get(dep) ?? dep).join(', ');
</script>

{#if actions && actions.length > 0}
  <article class="card">
    <header class="card-list-header">
      <h3 class="text-sm font-medium">
        {title} ({actions.length})
      </h3>
      {#if isDAG}
        <p class="text-lighter mode-note">
          {stages.length} {stages.length === 1 ? 'step' : 'steps'} · actions in a step run together
        </p>
      {/if}
    </header>

    {#if isDAG}
      <div class="stages">
        {#each stages as stage, stageIndex (stageIndex)}
          <div class="stage">
            <div class="step-label">
              Step {stageIndex + 1}
              {#if stage.length > 1}
                <span class="parallel">{stage.length} parallel</span>
              {/if}
            </div>
            <div class="stage-actions" class:branched={stage.length > 1}>
              {#each stage as action (action.id)}
                <div class="action-item">
                  <div class="action-body">
                    <div class="text-sm action-name">{action.name}</div>
                    {#if action.needs && action.needs.length > 0}
                      <div class="needs text-lighter">
                        needs {dependencyNames(action.needs)}
                      </div>
                    {/if}
                  </div>
                  <span class="badge primary shrink-0">
                    {action.executor}
                  </span>
                </div>
              {/each}
            </div>
          </div>
        {/each}
      </div>
    {:else}
      <div class="vstack gap-1">
        {#each actions as action, index (action.id)}
          <div class="action-item numbered">
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
    {/if}
  </article>
{/if}

<style>
  .card-list-header {
    padding: var(--space-3) var(--space-4);
    border-bottom: 1px solid var(--border);
  }
  .mode-note {
    margin: var(--space-1) 0 0;
    font-size: var(--text-8);
  }
  .action-item {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-3) var(--space-4);
  }
  .action-item.numbered {
    border-bottom: 1px solid var(--border);
  }
  .action-item.numbered:last-child {
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

  .stage {
    padding: var(--space-2) 0 var(--space-3);
  }
  .stage + .stage {
    border-top: 1px solid var(--border);
  }
  .step-label {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-4) 0;
    color: var(--muted-foreground);
    font-size: var(--text-8);
    letter-spacing: 0.04em;
    text-transform: uppercase;
  }
  .parallel {
    padding: 0 0.35rem;
    border-radius: var(--radius-full);
    background: var(--faint);
    letter-spacing: 0;
  }
  /* A rail marks the actions of one step as concurrent branches rather than a sequence. */
  .stage-actions.branched {
    margin-inline-start: var(--space-4);
    border-inline-start: 2px solid var(--border);
  }
  .stage-actions.branched .action-item {
    padding-inline-start: var(--space-3);
  }
  .action-body {
    flex: 1;
    min-width: 0;
  }
  .action-body .action-name {
    flex: none;
  }
  .needs {
    font-size: var(--text-8);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
