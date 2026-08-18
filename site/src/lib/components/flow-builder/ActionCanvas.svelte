<script lang="ts">
  import { IconPlus, IconCopy, IconX, IconLock, IconGitBranch, IconServer } from '@tabler/icons-svelte';
  import type { BuilderAction, ExecutionMode } from '$lib/types';
  import { ancestors, cyclicActions, dependencyPath, withImplicitNeeds } from '$lib/utils/dag';
  import { linkActions, nameLookup, unlinkActions } from '$lib/utils/flowBuilder';
  import { EDITOR_GEOMETRY as GEO, type PipelineLayout } from '$lib/utils/pipelineLayout';

  let {
    actions,
    layout,
    executionMode,
    selectedId = $bindable(),
    filter = '',
    fit = false,
    disabled = false,
    onInsert,
    onAddAfter,
    onDuplicate,
    onDelete
  }: {
    actions: BuilderAction[];
    layout: PipelineLayout<BuilderAction>;
    executionMode: ExecutionMode;
    selectedId?: string;
    filter?: string;
    fit?: boolean;
    disabled?: boolean;
    onInsert: (from: string, to: string) => void;
    onAddAfter: (id: string) => void;
    onDuplicate: (id: string) => void;
    onDelete: (id: string) => void;
  } = $props();

  let linkingFrom = $state<string | null>(null);
  let paneWidth = $state(0);

  const isDAG = $derived(executionMode === 'dag');
  const cyclic = $derived(isDAG ? cyclicActions(actions) : new Set<string>());
  const scale = $derived(
    fit && layout.width > 0 ? Math.min(1, (paneWidth - 48) / layout.width) : 1
  );

  const pathActions = $derived(withImplicitNeeds(actions, executionMode));
  const litPath = $derived(
    !linkingFrom && selectedId ? dependencyPath(pathActions, selectedId) : null
  );

  // Linking into an ancestor would close a cycle, so those recede while picking.
  const blocked = $derived(
    linkingFrom ? new Set([linkingFrom, ...ancestors(actions, linkingFrom)]) : null
  );

  const query = $derived(filter.trim().toLowerCase());
  const filteredOut = $derived(
    query
      ? new Set(
          actions
            .filter((a) => !`${a.name} ${a.executor}`.toLowerCase().includes(query))
            .map((a) => a.id)
        )
      : null
  );

  const nameOf = $derived(nameLookup(actions));

  const isDimmed = (id: string) => !!blocked?.has(id) || !!filteredOut?.has(id);
  const isInvalid = (a: BuilderAction) => !a.name?.trim() || !a.executor || cyclic.has(a.id);

  function pickNode(id: string) {
    if (linkingFrom) {
      if (!blocked?.has(id)) linkActions(actions, linkingFrom, id);
      linkingFrom = null;
    }
    selectedId = id;
  }

  function startLink(event: MouseEvent, id: string) {
    event.stopPropagation();
    linkingFrom = linkingFrom === id ? null : id;
  }

  function clearCanvas() {
    linkingFrom = null;
    selectedId = '';
  }
</script>

<svelte:window
  onkeydown={(e) => {
    if (e.key === 'Escape' && linkingFrom) linkingFrom = null;
  }}
/>

<div class="pane" bind:clientWidth={paneWidth}>
  {#if linkingFrom}
    <p class="link-banner" role="status">
      <IconGitBranch size={16} />
      Pick the action that should wait for <strong>{nameOf(linkingFrom)}</strong>
      <button type="button" class="ghost small" onclick={() => (linkingFrom = null)}>Cancel</button>
    </p>
  {/if}

  <div
    class="scroll"
    onclick={(e) => {
      if (e.target === e.currentTarget) clearCanvas();
    }}
    onkeydown={(e) => {
      if (e.key === 'Escape') clearCanvas();
    }}
    role="presentation"
  >
    <div class="fit" style="width: {layout.width * scale}px; height: {layout.height * scale}px">
      <div
        class="canvas"
        class:linking={!!linkingFrom}
        style="width: {layout.width}px; height: {layout.height}px; transform: scale({scale})"
      >
        <svg class="edges" width={layout.width} height={layout.height} aria-hidden="true">
          {#each layout.edges as edge (edge.from + '->' + edge.to)}
            {@const lit = litPath?.has(edge.from) && litPath?.has(edge.to)}
            <path class="edge" class:lit d={edge.path} />
          {/each}
        </svg>

        {#if !disabled}
          {#each layout.edges as edge (edge.from + '->' + edge.to)}
            <span class="edge-tools" style="left: {edge.midX}px; top: {edge.midY}px">
              <button
                type="button"
                class="ghost icon small"
                onclick={() => onInsert(edge.from, edge.to)}
                title="Insert an action here"
                aria-label="Insert an action between {nameOf(edge.from)} and {nameOf(edge.to)}"
              >
                <IconPlus size={13} />
              </button>
              {#if isDAG}
                <button
                  type="button"
                  data-variant="danger"
                  class="ghost icon small"
                  onclick={() => unlinkActions(actions, edge.from, edge.to)}
                  title="Remove this dependency"
                  aria-label="{nameOf(edge.to)} no longer waits for {nameOf(edge.from)}"
                >
                  <IconX size={13} />
                </button>
              {/if}
            </span>
          {/each}
        {/if}

        {#each layout.stages as stage (stage.index)}
          <div
            class="stage-label overline text-lighter"
            style="left: {stage.x}px; width: {GEO.nodeWidth}px; height: {GEO.stageHeader}px"
          >
            {isDAG ? 'Stage' : 'Step'}
            {stage.index + 1}
            {#if stage.nodes.length > 1}<span class="badge secondary small">{stage.nodes.length}</span>{/if}
          </div>

          <!-- Keyed by tempId: the id changes on every keystroke of a rename. -->
          {#each stage.nodes as node (node.action.tempId)}
            {@const action = node.action}
            <div
              class="node-wrap"
              style="left: {node.x}px; top: {node.y}px; width: {GEO.nodeWidth}px; height: {GEO.nodeHeight}px"
            >
              <button
                type="button"
                class="node"
                class:selected={selectedId === action.id}
                class:invalid={isInvalid(action)}
                class:dimmed={isDimmed(action.id)}
                onclick={() => pickNode(action.id)}
              >
                <span class="node-name" class:untitled={!action.name}>
                  {action.name || 'Untitled action'}
                </span>
                <span class="node-meta">
                  {#if action.executor}
                    <span class="badge outline small">{action.executor}</span>
                  {:else}
                    <span class="badge danger small">no executor</span>
                  {/if}
                  {#if action.approval}
                    <span title="Requires approval"><IconLock size={14} /></span>
                  {/if}
                  {#if action.selectedNodes.length > 0}
                    <span title="{action.selectedNodes.length} node(s)"><IconServer size={14} /></span>
                  {/if}
                </span>
              </button>

              <span class="port in"></span>
              {#if isDAG && !disabled}
                <button
                  type="button"
                  class="port out"
                  class:active={linkingFrom === action.id}
                  onclick={(e) => startLink(e, action.id)}
                  title="Click, then click the action that should wait for this one"
                  aria-label="Make another action wait for {action.name || action.id}"
                ></button>
              {/if}

              {#if !disabled}
                <span class="node-tools">
                  <button
                    type="button"
                    class="ghost icon small"
                    onclick={() => onAddAfter(action.id)}
                    title="Add an action after this one"
                    aria-label="Add an action after {action.name || action.id}"
                  >
                    <IconPlus size={14} />
                  </button>
                  <button
                    type="button"
                    class="ghost icon small"
                    onclick={() => onDuplicate(action.id)}
                    title="Duplicate"
                    aria-label="Duplicate {action.name || action.id}"
                  >
                    <IconCopy size={14} />
                  </button>
                  <button
                    type="button"
                    data-variant="danger"
                    class="ghost icon small"
                    onclick={() => onDelete(action.id)}
                    title="Delete"
                    aria-label="Delete {action.name || action.id}"
                  >
                    <IconX size={14} />
                  </button>
                </span>
              {/if}
            </div>
          {/each}
        {/each}
      </div>
    </div>
  </div>
</div>

<style>
  .pane {
    flex: 1;
    min-height: 0;
    min-width: 16rem;
    display: flex;
    flex-direction: column;
    position: relative;
  }

  .scroll {
    flex: 1;
    min-height: 0;
    overflow: auto;
    padding: var(--space-6);
    display: grid;
    align-content: safe center;
    justify-content: safe center;
    background-image: radial-gradient(var(--border) 1px, transparent 1px);
    background-size: 22px 22px;
  }

  .fit {
    position: relative;
    overflow: hidden;
  }

  .canvas {
    position: relative;
    transform-origin: top left;
  }

  .edges {
    position: absolute;
    inset: 0;
    overflow: visible;
    pointer-events: none;
  }

  .edge {
    fill: none;
    stroke: var(--border);
    stroke-width: 2;
    transition: stroke var(--transition-fast);
  }
  .edge.lit {
    stroke: var(--primary);
  }

  .link-banner {
    position: absolute;
    inset-block-start: var(--space-3);
    inset-inline: 0;
    z-index: 4;
    width: fit-content;
    margin: 0 auto;
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-1) var(--space-3);
    border-radius: var(--radius-full);
    background: var(--accent);
    color: var(--primary);
    font-size: var(--text-8);
  }

  .stage-label {
    position: absolute;
    inset-block-start: 0;
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }

  .node-wrap {
    position: absolute;
  }

  .node {
    all: unset;
    position: absolute;
    inset: 0;
    box-sizing: border-box;
    display: flex;
    flex-direction: column;
    justify-content: center;
    gap: var(--space-1);
    padding: var(--space-3);
    border: 2px solid var(--border);
    border-radius: var(--radius-medium);
    background: var(--card);
    color: var(--foreground);
    cursor: pointer;
    transition:
      border-color var(--transition-fast),
      box-shadow var(--transition-fast),
      opacity var(--transition-fast);
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
    box-shadow: var(--shadow-medium);
  }
  .node.invalid {
    border-color: var(--danger);
    border-style: dashed;
  }
  .node.dimmed {
    opacity: 0.4;
  }

  .node-name {
    font-size: var(--text-7);
    font-weight: var(--font-medium);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .node-name.untitled {
    color: var(--muted-foreground);
    font-style: italic;
  }
  .node-meta {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    font-size: var(--text-8);
    color: var(--muted-foreground);
    overflow: hidden;
  }

  .port {
    position: absolute;
    inset-block-start: 50%;
    width: 0.875rem;
    height: 0.875rem;
    margin: -0.4375rem 0 0 -0.4375rem;
    box-sizing: border-box;
    padding: 0;
    border: 2px solid var(--border);
    border-radius: var(--radius-full);
    background: var(--card);
    transition:
      transform var(--transition-fast),
      border-color var(--transition-fast);
  }
  .port.in {
    inset-inline-start: 0;
    pointer-events: none;
  }
  .port.out {
    inset-inline-start: 100%;
    cursor: crosshair;
  }
  .port.out:hover,
  .port.out:focus-visible {
    border-color: var(--primary);
    transform: scale(1.3);
    outline: none;
  }
  .port.out.active {
    border-color: var(--primary);
    background: var(--primary);
    transform: scale(1.3);
  }

  .canvas.linking .node:not(.dimmed) {
    border-color: var(--primary);
    border-style: dashed;
  }
  .canvas.linking .node-tools {
    display: none;
  }

  .node-tools,
  .edge-tools {
    position: absolute;
    display: flex;
    gap: 2px;
    padding: 2px;
    border: 1px solid var(--border);
    border-radius: var(--radius-medium);
    background: var(--card);
    box-shadow: var(--shadow-small);
    transition: opacity var(--transition-fast);
  }

  .node-tools {
    inset-block-start: calc(var(--space-3) * -1);
    inset-inline-end: var(--space-2);
    opacity: 0;
  }
  .node-wrap:hover .node-tools,
  .node-tools:focus-within,
  .node-wrap:has(.node.selected) .node-tools {
    opacity: 1;
  }

  .edge-tools {
    z-index: 2;
    transform: translate(-50%, -50%);
    opacity: 0.45;
  }
  .edge-tools:hover,
  .edge-tools:focus-within {
    opacity: 1;
  }

</style>
