<script lang="ts">
  import {
    IconX,
    IconCheck,
    IconPlayerPlay,
    IconClockPause,
    IconCircle,
    IconMinus,
    IconSearch,
    IconArrowRight,
    IconPlayerSkipForward,
  } from '@tabler/icons-svelte';
  import type { StepStatus } from '$lib/utils/dag';

  type Action = {
    id: string;
    name: string;
    status: StepStatus;
    level?: number;
    needs?: string[];
  };

  type Props = {
    actions: Action[];
    selectedActionId?: string;
    onActionSelect: (actionId: string) => void;
  };

  let {
    actions,
    selectedActionId = $bindable(),
    onActionSelect
  }: Props = $props();

  let searchQuery = $state('');

  const filteredActions = $derived(
    actions.filter(action =>
      action.name.toLowerCase().includes(searchQuery.toLowerCase())
    )
  );

  // Actions carry a level only in dag mode, where actions sharing one run together.
  const groups = $derived.by(() => {
    if (filteredActions.every(a => a.level === undefined)) {
      return [{ level: undefined, actions: filteredActions }];
    }

    const byLevel = new Map<number, Action[]>();
    for (const action of filteredActions) {
      const level = action.level ?? 0;
      byLevel.set(level, [...(byLevel.get(level) ?? []), action]);
    }
    return [...byLevel.entries()]
      .sort(([a], [b]) => a - b)
      .map(([level, actions]) => ({ level, actions }));
  });

  const nameById = $derived(new Map(actions.map(a => [a.id, a.name])));

  const getStatusClass = (status: StepStatus) => {
    switch (status) {
      case 'failed': return 'status-failed';
      case 'completed': return 'status-completed';
      case 'running': return 'status-running';
      case 'awaiting_approval': return 'status-waiting';
      case 'cancelled': return 'status-cancelled';
      case 'skipped': return 'status-skipped';
      default: return 'status-pending';
    }
  };

  const getIcon = (status: StepStatus) => {
    switch (status) {
      case 'failed': return IconX;
      case 'completed': return IconCheck;
      case 'running': return IconPlayerPlay;
      case 'awaiting_approval': return IconClockPause;
      case 'cancelled': return IconCircle;
      case 'skipped': return IconPlayerSkipForward;
      default: return IconMinus;
    }
  };

  const handleActionClick = (actionId: string) => {
    onActionSelect(actionId);
  };
</script>

<div class="actions-panel card" style="padding: 0;">
  <!-- Header with Search -->
  <div class="panel-header">
    <h2>Actions</h2>
    <fieldset class="group">
      <legend><IconSearch size={16} /></legend>
      <input
        type="search"
        bind:value={searchQuery}
        placeholder="Search actions..."
      />
    </fieldset>
  </div>

  <!-- Actions List -->
  <div class="actions-list">
    {#if filteredActions.length === 0}
      <div class="empty-msg text-lighter">
        {searchQuery ? 'No actions found' : 'No actions available'}
      </div>
    {:else}
      <div class="vstack gap-1">
        {#each groups as group (group.level ?? 'all')}
          {#if group.level !== undefined}
            <div class="level-label text-lighter">
              Step {group.level + 1}
              {#if group.actions.length > 1}
                <span>· {group.actions.length} in parallel</span>
              {/if}
            </div>
          {/if}
          {#each group.actions as action (action.id)}
            <button
              type="button"
              onclick={() => handleActionClick(action.id)}
              class="action-item {getStatusClass(action.status)}"
              class:selected={selectedActionId === action.id}
            >
              <div class="hstack gap-2 justify-between">
                <div class="action-name">{action.name}</div>
                <div class="status-icon {getStatusClass(action.status)}">
                  <svelte:component this={getIcon(action.status)} size={14} />
                </div>
              </div>
              {#if action.needs && action.needs.length > 0}
                <div class="action-needs text-lighter">
                  <IconArrowRight size={11} />
                  {action.needs.map((id) => nameById.get(id) ?? id).join(', ')}
                </div>
              {/if}
            </button>
          {/each}
        {/each}
      </div>
    {/if}
  </div>
</div>

<style>
  .actions-panel {
    display: flex;
    flex-direction: column;
    height: 100%;
    overflow: hidden;
  }
  .panel-header {
    flex-shrink: 0;
    position: sticky;
    top: 0;
    background: var(--card);
    border-bottom: 1px solid var(--border);
    padding: var(--space-3) var(--space-4);
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    z-index: 10;
  }
  .panel-header h2 {
    font-size: var(--text-6);
    font-weight: 600;
    color: var(--foreground);
    margin: 0;
  }
  .panel-header fieldset {
    margin: 0;
  }
  .actions-list {
    overflow-y: auto;
    padding: var(--space-2);
    flex: 1;
    min-height: 0;
  }
  .empty-msg {
    text-align: center;
    padding: var(--space-4) 0;
    font-size: var(--text-7);
  }
  .action-item {
    all: unset;
    display: block;
    box-sizing: border-box;
    width: 100%;
    padding: var(--space-3) var(--space-3);
    border-radius: var(--radius-medium);
    border: 2px solid var(--border);
    background: var(--card);
    color: var(--foreground);
    cursor: pointer;
    transition: all 0.15s;
  }
  .action-item.selected {
    border-color: var(--primary);
  }
  .action-name {
    font-weight: 500;
    font-size: var(--text-7);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    min-width: 0;
    flex: 1;
  }
  .status-icon {
    border-radius: 9999px;
    padding: 0.25rem;
    flex-shrink: 0;
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .status-failed {
    background: color-mix(in srgb, var(--danger) 10%, transparent);
    color: var(--danger);
  }
  .status-failed.status-icon {
    background: var(--danger);
    color: white;
  }
  .status-completed {
    background: color-mix(in srgb, var(--success) 10%, transparent);
    color: var(--success);
  }
  .status-completed.status-icon {
    background: var(--success);
    color: white;
  }
  .status-running {
    background: color-mix(in srgb, var(--primary) 10%, transparent);
    color: var(--primary);
  }
  .status-running.status-icon {
    background: var(--primary);
    color: white;
    animation: pulse 2s infinite;
  }
  .status-waiting {
    background: color-mix(in srgb, var(--warning) 10%, transparent);
    color: var(--warning);
  }
  .status-waiting.status-icon {
    background: var(--warning);
    color: white;
  }
  .status-cancelled {
    background: var(--faint);
    color: var(--foreground);
  }
  .status-cancelled.status-icon {
    background: #9ca3af;
    color: white;
  }
  .status-pending {
    background: var(--faint);
    color: var(--muted-foreground);
  }
  .status-pending.status-icon {
    background: #6b7280;
    color: white;
  }
  .status-skipped {
    background: var(--faint);
    color: var(--muted-foreground);
  }
  .status-skipped.status-icon {
    background: #9ca3af;
    color: white;
  }
  .level-label {
    font-size: var(--text-8);
    text-transform: uppercase;
    letter-spacing: 0.04em;
    padding: var(--space-2) var(--space-2) var(--space-1);
  }
  .action-needs {
    display: flex;
    align-items: center;
    gap: 0.2rem;
    font-size: var(--text-8);
    margin-top: 0.15rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  @keyframes pulse {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.5; }
  }
</style>
