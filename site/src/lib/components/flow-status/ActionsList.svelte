<script lang="ts">
  import {
    IconX,
    IconCheck,
    IconPlayerPlay,
    IconClockPause,
    IconMinus,
    IconPlayerSkipForward,
    IconSearch,
    IconCircle,
    IconCircleCheck,
    IconRefresh,
  } from '@tabler/icons-svelte';
  import type { StepStatus } from '$lib/utils/dag';

  type Action = {
    id: string;
    name: string;
    status: StepStatus;
    level?: number;
    needs?: string[];
    duration?: string;
    approval?: boolean;
  };

  let {
    actions,
    selectedActionId = $bindable(),
    onActionSelect,
    canRerun = false,
    onRerun,
  }: {
    actions: Action[];
    selectedActionId?: string;
    onActionSelect: (actionId: string) => void;
    canRerun?: boolean;
    onRerun?: (actionId: string) => void;
  } = $props();

  let searchQuery = $state('');

  const filteredActions = $derived(
    actions.filter((action) =>
      action.name.toLowerCase().includes(searchQuery.trim().toLowerCase())
    )
  );

  const groups = $derived.by(() => {
    if (filteredActions.every((action) => action.level === undefined)) {
      return filteredActions.map((action, index) => ({ level: index, actions: [action] }));
    }

    const byLevel = new Map<number, Action[]>();
    for (const action of filteredActions) {
      const level = action.level ?? 0;
      byLevel.set(level, [...(byLevel.get(level) ?? []), action]);
    }
    return [...byLevel.entries()]
      .sort(([a], [b]) => a - b)
      .map(([level, groupedActions]) => ({ level, actions: groupedActions }));
  });

  const statusClass = (status: StepStatus) => `status-${status}`;

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
</script>

<aside class="actions-panel" aria-label="Execution actions">
  <div class="panel-header">
    <fieldset class="group">
      <legend><IconSearch size={16} /></legend>
      <input type="search" bind:value={searchQuery} placeholder="Filter actions" />
    </fieldset>
  </div>

  <div class="actions-list">
    {#if filteredActions.length === 0}
      <div class="empty-msg text-lighter">
        {searchQuery ? 'No matching actions' : 'No actions available'}
      </div>
    {:else}
      {#each groups as group (group.level)}
        <div class="step-label">
          Step {group.level + 1}
          {#if group.actions.length > 1}
            <span class="parallel">{group.actions.length} parallel</span>
          {/if}
        </div>
        {#each group.actions as action (action.id)}
          {@const Icon = getIcon(action.status)}
          <div class="action-row">
            <button
              type="button"
              onclick={() => onActionSelect(action.id)}
              class="action-item"
              class:selected={selectedActionId === action.id}
              aria-current={selectedActionId === action.id ? 'true' : undefined}
            >
              <span class="status-dot {statusClass(action.status)}" title={action.status.replace('_', ' ')}>
                <Icon size={11} stroke={3} />
              </span>
              <span class="action-name">{action.name}</span>
              {#if action.duration}
                <span class="action-time">{action.duration}</span>
              {/if}
              {#if action.approval}
                <span
                  class="approval"
                  role="img"
                  aria-label="Approval required"
                  data-tooltip="Approval required"
                ><IconCircleCheck size={15} /></span>
              {/if}
            </button>
            {#if canRerun}
              <button
                type="button"
                class="rerun-action"
                onclick={() => onRerun?.(action.id)}
                aria-label={`Re-run from ${action.name}`}
                title="Re-run from here"
              >
                <IconRefresh size={15} />
              </button>
            {/if}
          </div>
        {/each}
      {/each}
    {/if}
  </div>
</aside>

<style>
  .actions-panel {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
    overflow: hidden;
    background: var(--card);
  }
  .panel-header {
    display: flex;
    align-items: center;
    flex-shrink: 0;
    height: var(--space-12);
    padding-inline: var(--space-3);
    background: var(--card);
    border-bottom: 1px solid var(--border);
  }
  .panel-header fieldset {
    width: 100%;
    margin: 0;
  }
  .panel-header input {
    margin: 0;
    padding-block: var(--space-1);
  }
  .actions-list {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    padding: var(--space-2);
  }
  .empty-msg {
    padding: var(--space-6) var(--space-2);
    text-align: center;
    font-size: var(--text-7);
  }
  .step-label {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-3) var(--space-2) var(--space-1);
    color: var(--muted-foreground);
    font-size: var(--text-8);
    letter-spacing: 0.04em;
    text-transform: uppercase;
  }
  .step-label:first-child { padding-top: var(--space-1); }
  .parallel {
    padding: 0 0.35rem;
    border-radius: var(--radius-full);
    background: var(--faint);
    letter-spacing: 0;
  }
  .action-item {
    all: unset;
    box-sizing: border-box;
    display: flex;
    align-items: center;
    gap: var(--space-2);
    flex: 1;
    min-width: 0;
    padding: var(--space-2);
    border-inline-start: 2px solid transparent;
    border-radius: var(--radius-small);
    color: var(--foreground);
    cursor: pointer;
    transition: background-color var(--transition-fast);
  }
  .action-row {
    display: flex;
    align-items: center;
    gap: var(--space-1);
  }
  .rerun-action {
    all: unset;
    box-sizing: border-box;
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
    width: 1.75rem;
    height: 1.75rem;
    border-radius: var(--radius-small);
    color: var(--muted-foreground);
    cursor: pointer;
  }
  .rerun-action:hover {
    background: var(--muted);
    color: var(--foreground);
  }
  .rerun-action:focus-visible {
    outline: 2px solid var(--ring);
  }
  .action-item:hover { background: var(--muted); }
  .action-item.selected {
    background: var(--accent);
    border-inline-start-color: var(--primary);
  }
  .action-item:focus-visible {
    outline: 2px solid var(--ring);
    outline-offset: -2px;
  }
  .status-dot {
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
    width: 1.1rem;
    height: 1.1rem;
    border-radius: var(--radius-full);
    color: var(--background);
  }
  .status-completed { background: var(--success); }
  .status-failed { background: var(--danger); }
  .status-running { background: var(--primary); animation: pulse 2s infinite; }
  .status-awaiting_approval { background: var(--warning); }
  .status-cancelled,
  .status-skipped,
  .status-pending { background: var(--faint-foreground); }
  .action-name {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: var(--text-7);
    font-weight: var(--font-medium);
  }
  .action-time {
    flex-shrink: 0;
    color: var(--muted-foreground);
    font-size: var(--text-8);
    font-variant-numeric: tabular-nums;
  }
  .approval {
    display: flex;
    flex-shrink: 0;
    color: var(--warning);
  }
  .approval[data-tooltip]::before,
  .approval[data-tooltip]::after {
    inset-inline-start: auto;
    inset-inline-end: 0;
    transform: translateY(4px);
  }
  .approval[data-tooltip]:hover::before,
  .approval[data-tooltip]:hover::after {
    transform: translateY(0);
  }
  @keyframes pulse {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.5; }
  }
</style>
