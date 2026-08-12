<script lang="ts">
  import {
    IconX,
    IconCheck,
    IconPlayerPlay,
    IconClockPause,
    IconCircle,
    IconMinus,
    IconPlayerSkipForward,
  } from '@tabler/icons-svelte';
  import { dependencyPath, type StepStatus } from '$lib/utils/dag';
  import {
    pipelineLayout,
    NODE_WIDTH,
    NODE_HEIGHT,
    STAGE_HEADER,
  } from '$lib/utils/pipelineLayout';

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
    selectedActionId = $bindable(),
    onActionSelect,
  }: {
    actions: GraphAction[];
    statuses?: Record<string, StepStatus>;
    retries?: Record<string, number>;
    selectedActionId?: string;
    onActionSelect?: (actionId: string) => void;
  } = $props();

  let hoveredId = $state<string | null>(null);

  const layout = $derived(pipelineLayout(actions));

  const MAX_VIEWPORT = 380;
  const SCROLL_PADDING = 40;

  // Fit short pipelines, cap tall ones so the logs below stay on screen.
  const viewportHeight = $derived(
    Math.min(layout.height + SCROLL_PADDING, MAX_VIEWPORT)
  );

  // Dim everything outside the hovered action's dependency path, the way a pipeline view narrows
  // down to the chain you are pointing at.
  const highlighted = $derived(
    hoveredId ? dependencyPath(actions, hoveredId) : null
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
  <div class="graph-scroll" style="height: {viewportHeight}px;">
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
          class="stage-label text-lighter"
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
          {@const Icon = getIcon(status)}
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
            <span class="node-icon {getStatusClass(status)}">
              <Icon size={13} />
            </span>
            <span class="node-text">
              <span class="node-name">{node.action.name}</span>
              <span class="node-meta text-lighter">
                {node.action.executor || 'no executor'}
                {#if attempts > 1}
                  · attempt {attempts}
                {/if}
              </span>
            </span>
          </button>
        {/each}
      {/each}
    </div>
  </div>
{/if}

<style>
  .graph-scroll {
    /* A wide pipeline scrolls sideways rather than being clipped */
    overflow-x: auto;
    overflow-y: auto;
    /* Height comes from the content, capped inline. No max-height, so dragging the bottom edge can
       open a big graph up past the cap. */
    resize: vertical;
    min-height: 7rem;
    padding: var(--space-3) var(--space-4) var(--space-4);
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
    font-size: var(--text-8);
    text-transform: uppercase;
    letter-spacing: 0.04em;
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
    gap: var(--space-2);
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
    box-shadow: 0 1px 6px rgb(0 0 0 / 0.12);
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

  .node-icon {
    flex-shrink: 0;
    width: 1.25rem;
    height: 1.25rem;
    border-radius: 999px;
    display: flex;
    align-items: center;
    justify-content: center;
    color: white;
  }
  .node-text {
    min-width: 0;
    display: flex;
    flex-direction: column;
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

  .status-completed.node-icon {
    background: var(--success);
  }
  .status-completed.node {
    border-color: color-mix(in srgb, var(--success) 45%, var(--border));
  }
  .status-failed.node-icon {
    background: var(--danger);
  }
  .status-failed.node {
    border-color: color-mix(in srgb, var(--danger) 55%, var(--border));
  }
  .status-running.node-icon {
    background: var(--primary);
    animation: pulse 2s infinite;
  }
  .status-running.node {
    border-color: color-mix(in srgb, var(--primary) 55%, var(--border));
  }
  .status-waiting.node-icon {
    background: var(--warning);
  }
  .status-waiting.node {
    border-color: color-mix(in srgb, var(--warning) 55%, var(--border));
  }
  .status-cancelled.node-icon {
    background: #9ca3af;
  }
  .status-skipped.node-icon {
    background: #9ca3af;
  }
  .status-skipped.node {
    border-style: dashed;
  }
  .status-pending.node-icon {
    background: #6b7280;
  }

  @keyframes pulse {
    0%,
    100% {
      opacity: 1;
    }
    50% {
      opacity: 0.5;
    }
  }
</style>
