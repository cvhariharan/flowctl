<script lang="ts">
  import {
    IconX,
    IconCopy,
    IconTrash,
    IconChevronsLeft,
    IconChevronsRight
  } from '@tabler/icons-svelte';
  import OatSelect from '$lib/components/shared/OatSelect.svelte';
  import NodeSelector from '$lib/components/shared/NodeSelector.svelte';
  import KeyValueEditor from '$lib/components/shared/KeyValueEditor.svelte';
  import ExecutorFields from './ExecutorFields.svelte';
  import type { BuilderAction } from '$lib/types';
  import { descendants } from '$lib/utils/dag';
  import { linkActions, nameLookup, unlinkActions } from '$lib/utils/flowBuilder';

  let {
    action,
    actions,
    namespace,
    availableExecutors,
    executorConfigs,
    isDAG,
    disabled = false,
    onRename,
    onExecutorChange,
    onDuplicate,
    onDelete,
    onClose
  }: {
    action: BuilderAction | undefined;
    actions: BuilderAction[];
    namespace: string;
    availableExecutors: Array<{ name: string; capabilities: string[] }>;
    executorConfigs: Record<string, any>;
    isDAG: boolean;
    disabled?: boolean;
    onRename: (action: BuilderAction, name: string) => void;
    onExecutorChange: (action: BuilderAction, executor: string) => void;
    onDuplicate: (id: string) => void;
    onDelete: (id: string) => void;
    onClose: () => void;
  } = $props();

  let panel = $state<HTMLElement>();
  let pendingDependency = $state('');
  const NARROW = 400;

  let width = $state(NARROW);
  const expanded = $derived(width > NARROW);

  const schema = $derived(action?.executor ? executorConfigs[action.executor] : undefined);
  const supportsRemote = $derived(
    availableExecutors
      .find((e) => e.name === action?.executor)
      ?.capabilities?.includes('remote_execution') ?? false
  );

  const executorOptions = $derived(
    availableExecutors.map((e) => ({ value: e.name, label: e.name }))
  );

  // Anything downstream of this action would close a cycle, so it cannot be a dependency.
  const downstream = $derived(action ? descendants(actions, action.id) : new Set<string>());
  const dependencyOptions = $derived(
    action
      ? actions
          .filter(
            (a) =>
              a.id && a.id !== action.id && !downstream.has(a.id) && !action.needs.includes(a.id)
          )
          .map((a) => ({ value: a.id, label: a.name || a.id }))
      : []
  );

  const nameOf = $derived(nameLookup(actions));

  function toggleWidth() {
    const available = panel?.parentElement?.clientWidth ?? 0;
    width = expanded ? NARROW : Math.round(available * 0.65);
  }

  function startResize(event: PointerEvent) {
    const handle = event.currentTarget as HTMLElement;
    const available = panel?.parentElement?.clientWidth ?? 0;
    const startX = event.clientX;
    const startWidth = width;

    event.preventDefault();
    handle.setPointerCapture(event.pointerId);

    const onMove = (move: PointerEvent) => {
      width = Math.min(Math.max(320, startWidth + (startX - move.clientX)), available - 320);
    };
    const stop = () => {
      handle.removeEventListener('pointermove', onMove);
      handle.removeEventListener('pointerup', stop);
      handle.removeEventListener('pointercancel', stop);
    };

    handle.addEventListener('pointermove', onMove);
    handle.addEventListener('pointerup', stop);
    handle.addEventListener('pointercancel', stop);
  }
</script>

<aside class="inspector" bind:this={panel} style="width: {width}px">
  <div
    class="resize"
    role="separator"
    aria-orientation="vertical"
    aria-label="Resize panel"
    onpointerdown={startResize}
  ></div>

  {#if !action}
    <header class="head"><h3>Nothing selected</h3></header>
    <div class="body">
      <p class="align-center text-lighter p-4">Pick an action to edit it.</p>
    </div>
  {:else}
    <header class="head">
      <div class="title">
        <h3>{action.name || 'Untitled action'}</h3>
        <span class="text-lighter text-xs">
          <code>{action.id || '—'}</code>{action.executor ? ` · ${action.executor}` : ''}
        </span>
      </div>
      <button
        type="button"
        class="ghost icon small"
        onclick={toggleWidth}
        aria-label={expanded ? 'Narrow panel' : 'Widen panel'}
        title={expanded ? 'Narrow the panel' : 'Widen the panel'}
      >
        {#if expanded}<IconChevronsRight size={16} />{:else}<IconChevronsLeft size={16} />{/if}
      </button>
      <button
        type="button"
        class="ghost icon small"
        onclick={onClose}
        aria-label="Close panel"
      >
        <IconX size={16} />
      </button>
    </header>

    <div class="body">
      <div class="group">
        <div data-field>
          <label for="action-name">Name <span class="req">*</span></label>
          <input
            id="action-name"
            type="text"
            {disabled}
            value={action.name}
            oninput={(e) => onRename(action, e.currentTarget.value)}
            placeholder="Run tests"
          />
        </div>
        <div data-field>
          <label for="action-executor">Executor <span class="req">*</span></label>
          <OatSelect
            value={action.executor}
            options={executorOptions}
            placeholder="Select an executor"
            id="action-executor"
            {disabled}
            onchange={(executor) => onExecutorChange(action, executor)}
          />
        </div>
      </div>

      {#if isDAG}
        <div class="group">
          <h4 class="overline">Waits for</h4>
          {#if action.needs.length > 0}
            <div class="hstack gap-2">
              {#each action.needs as dep (dep)}
                <span class="badge secondary">
                  {nameOf(dep)}
                  {#if !disabled}
                    <button
                      type="button"
                      onclick={() => unlinkActions(actions, dep, action.id)}
                      aria-label="Remove dependency on {nameOf(dep)}"
                    >&times;</button>
                  {/if}
                </span>
              {/each}
            </div>
          {:else}
            <p class="text-lighter text-xs">Nothing</p>
          {/if}
          {#if !disabled && dependencyOptions.length > 0}
            <OatSelect
              bind:value={pendingDependency}
              options={dependencyOptions}
              placeholder="Add a dependency..."
              id="action-needs"
              onchange={() => {
                if (pendingDependency) linkActions(actions, pendingDependency, action.id);
                pendingDependency = '';
              }}
            />
          {/if}
        </div>
      {/if}

      {#if schema}
        <div class="group">
          <h4 class="overline">{action.executor} configuration</h4>
          <ExecutorFields
            {schema}
            bind:values={action.with}
            idPrefix={`with-${action.tempId}`}
            codeHeight={expanded ? '420px' : '260px'}
            {disabled}
          />
        </div>
      {/if}

      {#if supportsRemote}
        <div class="group">
          <h4 class="overline">Where it runs</h4>
          <div data-field>
            <label for="action-nodes">Nodes</label>
            <NodeSelector
              {namespace}
              bind:selectedNodes={action.selectedNodes}
              placeholder="Search nodes..."
              {disabled}
            />
          </div>
          <label class="hstack gap-2">
            <input
              type="checkbox"
              role="switch"
              bind:checked={action.allow_node_override}
              {disabled}
            />
            <span>Let the run form choose the nodes</span>
          </label>
        </div>
      {/if}

      <div class="group">
        <h4 class="overline">Environment</h4>
        <KeyValueEditor
          bind:pairs={action.variables}
          keyPlaceholder="VAR_NAME"
          valuePlaceholder={'{{ inputs.name }}'}
          {disabled}
        />
      </div>

      <div class="group">
        <label class="hstack gap-2">
          <input type="checkbox" role="switch" bind:checked={action.approval} {disabled} />
          <span>Require approval before this action runs</span>
        </label>
      </div>

    </div>

    {#if !disabled}
      <footer class="foot">
        <button type="button" data-variant="secondary" class="small" onclick={() => onDuplicate(action.id)}>
          <IconCopy size={14} /> Duplicate
        </button>
        <button type="button" data-variant="danger" class="ghost small" onclick={() => onDelete(action.id)}>
          <IconTrash size={14} /> Delete action
        </button>
      </footer>
    {/if}
  {/if}
</aside>

<style>
  .inspector {
    position: relative;
    flex-shrink: 0;
    min-height: 0;
    display: flex;
    flex-direction: column;
    background: var(--card);
    border-inline-start: 1px solid var(--border);
  }

  /* Drag the seam to give a script more room. */
  .resize {
    position: absolute;
    inset-block: 0;
    inset-inline-start: -3px;
    width: 7px;
    z-index: 3;
    cursor: col-resize;
    touch-action: none;
  }
  .resize:hover {
    background: color-mix(in srgb, var(--primary) 35%, transparent);
  }

  .head,
  .foot {
    flex-shrink: 0;
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-4);
  }
  .head {
    border-bottom: 1px solid var(--border);
  }
  .foot {
    border-top: 1px solid var(--border);
  }
  .head h3 {
    margin: 0;
    font-size: var(--text-6);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .body {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    padding: var(--space-4);
  }
  /* Groups own their spacing; a flex column here would shrink them instead of scrolling. */
  .group + .group {
    margin-block-start: var(--space-5);
    padding-block-start: var(--space-5);
    border-top: 1px solid var(--border);
  }
  .group > h4 {
    margin: 0 0 var(--space-3);
    color: var(--muted-foreground);
  }
  .group > :last-child {
    margin-block-end: 0;
  }
</style>
