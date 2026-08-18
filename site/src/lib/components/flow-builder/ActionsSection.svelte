<script lang="ts">
  import { tick } from 'svelte';
  import IconPlus from '@tabler/icons-svelte/icons/plus';
  import IconMaximize from '@tabler/icons-svelte/icons/maximize';
  import { apiClient } from '$lib/apiClient';
  import { handleInlineError } from '$lib/utils/errorHandling';
  import type { BuilderAction, BuilderFlow, ExecutionMode } from '$lib/types';
  import { pipelineLayout, EDITOR_GEOMETRY } from '$lib/utils/pipelineLayout';
  import {
    addAfter,
    duplicateAction,
    insertBetween,
    newAction,
    removeAction,
    renameAction,
    resolveSchema
  } from '$lib/utils/flowBuilder';
  import ActionCanvas from './ActionCanvas.svelte';
  import ActionInspector from './ActionInspector.svelte';

  let {
    flow = $bindable(),
    namespace,
    availableExecutors,
    executorConfigs = $bindable(),
    selectedId = $bindable(''),
    disabled = false
  }: {
    flow: BuilderFlow;
    namespace: string;
    availableExecutors: Array<{ name: string; capabilities: string[] }>;
    executorConfigs: Record<string, any>;
    selectedId?: string;
    disabled?: boolean;
  } = $props();

  let filter = $state('');
  let fit = $state(false);

  const executionMode = $derived((flow.metadata.execution_mode || 'sequential') as ExecutionMode);
  const isDAG = $derived(executionMode === 'dag');
  const selected = $derived(flow.actions.find((a) => a.id === selectedId));
  const layout = $derived(pipelineLayout(flow.actions, executionMode, EDITOR_GEOMETRY));

  async function focusNew(action: BuilderAction) {
    selectedId = action.id;
    await tick();
    document.getElementById('action-name')?.focus();
  }

  function add() {
    const action = newAction('', isDAG && selectedId ? [selectedId] : []);
    flow.actions.push(action);
    focusNew(action);
  }

  function remove(id: string) {
    removeAction(flow.actions, id);
    if (selectedId === id) selectedId = '';
  }

  async function changeExecutor(action: BuilderAction, executor: string) {
    if (executor === action.executor) return;

    if (action.executor) action.withByExecutor[action.executor] = action.with;
    action.executor = executor;
    action.with = action.withByExecutor[executor] ?? {};

    if (!executor) return;

    const info = availableExecutors.find((e) => e.name === executor);
    if (!info?.capabilities?.includes('remote_execution')) {
      action.allow_node_override = false;
    }

    try {
      const schema =
        executorConfigs[executor] ?? resolveSchema(await apiClient.executors.getConfig(executor));

      executorConfigs[executor] = schema;
      for (const [key, property] of Object.entries(schema.properties ?? {}) as [string, any][]) {
        if (property.default !== undefined && action.with[key] === undefined) {
          action.with[key] = property.default;
        }
      }
    } catch (error) {
      handleInlineError(error, 'Unable to Load Executor Configuration');
    }
  }

  /** Arrow keys walk the graph: a dependency, a dependent, or a neighbour in the same stage. */
  function onKeydown(event: KeyboardEvent) {
    const target = event.target as HTMLElement | null;
    if (!selected || target?.closest('input, textarea, select, [contenteditable]')) return;

    if (event.key === 'Delete' && !disabled) {
      event.preventDefault();
      remove(selected.id);
      return;
    }
    if (!event.key.startsWith('Arrow')) return;

    const index = flow.actions.indexOf(selected);
    const stage = layout.stages.find((s) => s.nodes.some((n) => n.action.id === selected.id));
    const row = stage?.nodes.findIndex((n) => n.action.id === selected.id) ?? -1;
    const child = flow.actions.find((a) => a.needs.includes(selected.id));

    const next = {
      ArrowLeft: isDAG ? selected.needs[0] : flow.actions[index - 1]?.id,
      ArrowRight: isDAG ? child?.id : flow.actions[index + 1]?.id,
      ArrowUp: stage?.nodes[row - 1]?.action.id,
      ArrowDown: stage?.nodes[row + 1]?.action.id
    }[event.key];

    if (next) {
      event.preventDefault();
      selectedId = next;
    }
  }
</script>

<svelte:window onkeydown={onKeydown} />

<div class="section">
  <div class="toolbar hstack">
    {#if !disabled}
      <button type="button" class="small" onclick={add}>
        <IconPlus size={14} /> Add action
      </button>
    {/if}
    <span class="hint text-xs text-lighter">
      {#if isDAG && selected?.name}
        New actions wait for <strong>{selected.name}</strong>
      {/if}
    </span>

    <div class="toolbar-right hstack gap-4">
      {#if flow.actions.length > 3}
        <input
          type="search"
          class="search"
          bind:value={filter}
          placeholder="Find an action..."
          aria-label="Filter actions"
        />
      {/if}
      {#if layout.width > 900}
        <button
          type="button"
          class="small"
          data-variant="secondary"
          aria-pressed={fit}
          onclick={() => (fit = !fit)}
          title="Scale the graph to fit"
        >
          <IconMaximize size={14} /> Fit
        </button>
      {/if}
    </div>
  </div>

  <div class="workspace">
    {#if flow.actions.length === 0}
      <div class="empty-pane">
        <div class="card align-center">
          <h3>No actions yet</h3>
          <p class="text-lighter mb-4">A flow needs at least one action.</p>
          {#if !disabled}
            <button type="button" onclick={add}><IconPlus size={16} /> Add action</button>
          {/if}
        </div>
      </div>
    {:else}
      <ActionCanvas
        actions={flow.actions}
        {layout}
        {executionMode}
        bind:selectedId
        {filter}
        {fit}
        {disabled}
        onInsert={(from, to) => focusNew(insertBetween(flow.actions, from, to, isDAG))}
        onAddAfter={(id) => focusNew(addAfter(flow.actions, id, isDAG))}
        onDuplicate={(id) => (selectedId = duplicateAction(flow.actions, id))}
        onDelete={remove}
      />
    {/if}

    <ActionInspector
      action={selected}
      actions={flow.actions}
      {namespace}
      {availableExecutors}
      {executorConfigs}
      {isDAG}
      {disabled}
      onRename={(action, name) => {
        renameAction(flow.actions, action, name);
        selectedId = action.id;
      }}
      onExecutorChange={changeExecutor}
      onDuplicate={(id) => (selectedId = duplicateAction(flow.actions, id))}
      onDelete={remove}
      onClose={() => (selectedId = '')}
    />
  </div>
</div>

<style>
  .section,
  .workspace {
    flex: 1;
    min-height: 0;
    display: flex;
  }
  .section {
    flex-direction: column;
  }

  .toolbar {
    flex-shrink: 0;
    row-gap: var(--space-2);
    padding: var(--space-3) var(--space-4);
    background: var(--card);
    border-bottom: 1px solid var(--border);
  }
  .toolbar .hint {
    flex: 1 1 11rem;
    min-width: 0;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .toolbar-right {
    margin-inline-start: auto;
  }
  .search {
    width: 12rem;
    font-size: var(--text-8);
    padding-block: var(--space-1);
  }
  .empty-pane {
    flex: 1;
    min-width: 0;
    display: grid;
    place-items: center;
    padding: var(--space-8);
    background-image: radial-gradient(var(--border) 1px, transparent 1px);
    background-size: 22px 22px;
  }
  .empty-pane .card {
    max-width: 24rem;
  }

  @media (max-width: 1000px) {
    .workspace {
      flex-direction: column;
    }
  }
</style>
