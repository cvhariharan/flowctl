<script lang="ts">
  import { dependencyPath, withImplicitNeeds, type StepStatus } from '$lib/utils/dag';
  import type { ExecutionMode } from '$lib/types';
  import IconRefresh from '@tabler/icons-svelte/icons/refresh';
  import { pipelineLayout, DEFAULT_GEOMETRY } from '$lib/utils/pipelineLayout';

  const {
    nodeWidth: NODE_WIDTH,
    nodeHeight: NODE_HEIGHT,
    stageHeader: STAGE_HEADER,
  } = DEFAULT_GEOMETRY;

  type GraphAction = {
    id: string;
    name: string;
    executor?: string;
    needs?: string[];
  };

  let {
    actions,
    statuses = {},
    retries = {},
    durations = {},
    executionMode = 'dag',
    selectedActionId = $bindable(),
    onActionSelect,
    canRerun = false,
    onRerun,
  }: {
    actions: GraphAction[];
    statuses?: Record<string, StepStatus>;
    retries?: Record<string, number>;
    durations?: Record<string, string>;
    executionMode?: ExecutionMode;
    selectedActionId?: string;
    onActionSelect?: (actionId: string) => void;
    canRerun?: boolean;
    onRerun?: (actionId: string) => void;
  } = $props();

  let hoveredId = $state<string | null>(null);

  const layout = $derived(pipelineLayout(actions, executionMode));

  // Dim everything outside the hovered action's dependency path, the way a pipeline view narrows
  // down to the chain you are pointing at.
  const pathActions = $derived(withImplicitNeeds(actions, executionMode));
  const highlighted = $derived(
    hoveredId ? dependencyPath(pathActions, hoveredId) : null
  );

  const statusOf = (id: string): StepStatus => statuses[id] ?? 'pending';

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

  const edgeClass = (from: string, to: string) => {
    const status = statusOf(from);
    if (status === 'completed') return 'edge-done';
    if (status === 'failed' || status === 'cancelled') return 'edge-failed';
    if (statusOf(to) === 'skipped') return 'edge-skipped';
    return '';
  };

  const isDimmed = (id: string) => highlighted !== null && !highlighted.has(id);
</script>

{#if actions.length > 0}
  <div class="graph-scroll">
    <div
      class="graph-canvas"
      style="width: {layout.width}px; height: {layout.height}px;"
    >
      <svg
        class="edges"
        width={layout.width}
        height={layout.height}
        aria-hidden="true"
      >
        {#each layout.edges as edge (edge.from + '->' + edge.to)}
          <path
            d={edge.path}
            class="edge {edgeClass(edge.from, edge.to)}"
            class:edge-dimmed={isDimmed(edge.from) || isDimmed(edge.to)}
            class:edge-active={highlighted !== null &&
              highlighted.has(edge.from) &&
              highlighted.has(edge.to)}
          />
        {/each}
      </svg>

      {#each layout.stages as stage (stage.index)}
        <div
          class="stage-label overline text-lighter"
          style="left: {stage.x}px; width: {NODE_WIDTH}px; height: {STAGE_HEADER}px;"
        >
          Step {stage.index + 1}
          {#if stage.nodes.length > 1}
            <span class="stage-count">{stage.nodes.length}</span>
          {/if}
        </div>

        {#each stage.nodes as node (node.action.id)}
          {@const status = statusOf(node.action.id)}
          {@const attempts = retries[node.action.id] ?? 0}
          <button
            type="button"
            class="node {getStatusClass(status)}"
            class:selected={selectedActionId === node.action.id}
            class:dimmed={isDimmed(node.action.id)}
            style="left: {node.x}px; top: {node.y}px; width: {NODE_WIDTH}px; height: {NODE_HEIGHT}px;"
            onclick={() => onActionSelect?.(node.action.id)}
            onmouseenter={() => (hoveredId = node.action.id)}
            onmouseleave={() => (hoveredId = null)}
            onfocus={() => (hoveredId = node.action.id)}
            onblur={() => (hoveredId = null)}
            title="{node.action.name}{node.action.needs?.length
              ? ` — needs ${node.action.needs.join(', ')}`
              : ''}"
          >
            <span class="node-text">
              <span class="node-name">{node.action.name}</span>
              <span class="node-meta text-lighter">
                {node.action.executor || 'no executor'}
                {#if durations[node.action.id]}
                  · {durations[node.action.id]}
                {/if}
                {#if attempts > 1}
                  · attempt {attempts}
                {/if}
              </span>
            </span>
          </button>
          {#if canRerun}
            <button
              type="button"
              class="rerun-node"
              style="left: {node.x + NODE_WIDTH - 29}px; top: {node.y + 5}px;"
              onclick={() => onRerun?.(node.action.id)}
              aria-label={`Re-run from ${node.action.name}`}
              title="Re-run from here"
            >
              <IconRefresh size={14} />
            </button>
          {/if}
        {/each}
      {/each}
    </div>
  </div>
{/if}

<style>
  .graph-scroll {
    flex: 1;
    width: 100%;
    height: 100%;
    min-height: 0;
    overflow: auto;
    padding: var(--space-6) var(--space-4);
  }
  .graph-canvas {
    position: relative;
    /* auto margins centre a narrow graph and collapse to 0 when it overflows, which keeps the
       leftmost stage reachable (flex centring would clip it) */
    margin: 0 auto;
  }
  .edges {
    position: absolute;
    left: 0;
    top: 0;
    pointer-events: none;
    overflow: visible;
  }
  .edge {
    fill: none;
    stroke: var(--border);
    stroke-width: 2;
    transition:
      stroke 0.15s,
      opacity 0.15s;
  }
  .edge-done {
    stroke: color-mix(in srgb, var(--success) 55%, var(--border));
  }
  .edge-failed {
    stroke: color-mix(in srgb, var(--danger) 55%, var(--border));
  }
  .edge-skipped {
    stroke-dasharray: 4 4;
  }
  .edge-active {
    stroke-width: 2.5;
  }
  .edge-dimmed {
    opacity: 0.25;
  }

  .stage-label {
    position: absolute;
    top: 0;
    display: flex;
    align-items: center;
    gap: 0.375rem;
  }
  .stage-count {
    background: var(--faint);
    border-radius: 999px;
    padding: 0 0.35rem;
    letter-spacing: 0;
  }

  .node {
    all: unset;
    position: absolute;
    box-sizing: border-box;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 0 var(--space-3);
    border: 2px solid var(--border);
    border-radius: var(--radius-medium);
    background: var(--card);
    color: var(--foreground);
    cursor: pointer;
    transition:
      border-color 0.15s,
      opacity 0.15s,
      box-shadow 0.15s;
  }
  .node:hover {
    box-shadow: var(--shadow-small);
  }
  .node:focus-visible {
    outline: 2px solid var(--primary);
    outline-offset: 2px;
  }
  .node.selected {
    border-color: var(--primary);
  }
  .node.dimmed {
    opacity: 0.35;
  }
  .rerun-node {
    all: unset;
    position: absolute;
    z-index: 2;
    box-sizing: border-box;
    display: flex;
    align-items: center;
    justify-content: center;
    width: 1.5rem;
    height: 1.5rem;
    border-radius: var(--radius-small);
    color: var(--muted-foreground);
    cursor: pointer;
  }
  .rerun-node:hover {
    background: var(--muted);
    color: var(--foreground);
  }
  .rerun-node:focus-visible {
    outline: 2px solid var(--primary);
  }

  .node-text {
    min-width: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    line-height: 1.25;
  }
  .node-name {
    font-size: var(--text-7);
    font-weight: 500;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .node-meta {
    font-size: var(--text-8);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .status-completed.node {
    border-color: color-mix(in srgb, var(--success) 45%, var(--border));
  }
  .status-failed.node {
    border-color: color-mix(in srgb, var(--danger) 55%, var(--border));
  }
  .status-running.node {
    border-color: color-mix(in srgb, var(--primary) 55%, var(--border));
  }
  .status-waiting.node {
    border-color: color-mix(in srgb, var(--warning) 55%, var(--border));
  }
  .status-skipped.node {
    border-style: dashed;
  }
</style>
