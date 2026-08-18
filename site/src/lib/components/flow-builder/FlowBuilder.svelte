<script lang="ts">
  import {
    IconAlertTriangle,
    IconBell,
    IconBolt,
    IconLock,
    IconPlus,
    IconList
  } from '@tabler/icons-svelte';
  import FlowNotifications from '$lib/components/flow-create/FlowNotifications.svelte';
  import SecretsTab from '$lib/components/secrets/SecretsTab.svelte';
  import { onMount } from 'svelte';
  import type { BuilderFlow, BuilderSection } from '$lib/types';
  import { loadExecutorConfigs } from '$lib/utils/flowBuilder';
  import { validateFlow } from '$lib/utils/flowValidation';
  import ActionsSection from './ActionsSection.svelte';
  import GeneralSection from './GeneralSection.svelte';
  import InputsSection from './InputsSection.svelte';
  import ProblemStrip from './ProblemStrip.svelte';

  let {
    flow = $bindable(),
    namespace,
    mode,
    flowId = '',
    saving = false,
    readonly = false,
    availableExecutors,
    availableMessengers,
    messengerConfigs,
    onSave,
    onCancel
  }: {
    flow: BuilderFlow;
    namespace: string;
    mode: 'create' | 'edit';
    flowId?: string;
    saving?: boolean;
    readonly?: boolean;
    availableExecutors: Array<{ name: string; capabilities: string[] }>;
    availableMessengers: string[];
    messengerConfigs: Record<string, any>;
    onSave: () => void;
    onCancel: () => void;
  } = $props();

  const SECTIONS: Array<{ key: BuilderSection; label: string; icon: any }> = [
    { key: 'general', label: 'General', icon: IconList },
    { key: 'inputs', label: 'Inputs', icon: IconPlus },
    { key: 'actions', label: 'Actions', icon: IconBolt },
    { key: 'notify', label: 'Notifications', icon: IconBell },
    { key: 'secrets', label: 'Secrets', icon: IconLock }
  ];

  let section = $state<BuilderSection>(mode === 'create' ? 'general' : 'actions');
  let selectedActionId = $state('');
  let showProblems = $state(false);
  let executorConfigs = $state({} as Record<string, any>);

  onMount(async () => {
    executorConfigs = await loadExecutorConfigs(flow.actions);
  });

  const problems = $derived(validateFlow(flow, executorConfigs));
  const errors = $derived(problems.filter((p) => p.severity === 'error'));
  const problemsOpen = $derived(showProblems && errors.length > 0);
  const counts: Partial<Record<BuilderSection, number>> = $derived({
    inputs: flow.inputs.length,
    actions: flow.actions.length,
    notify: flow.notifications.length
  });

  const errorCounts = $derived(
    errors.reduce((acc, problem) => {
      acc[problem.section] = (acc[problem.section] ?? 0) + 1;
      return acc;
    }, {} as Partial<Record<BuilderSection, number>>)
  );

  function showProblem(actionId: string) {
    selectedActionId = actionId;
    section = 'actions';
  }
</script>

<div class="builder">
  <header class="bar hstack gap-4">
    <div class="title">
      <h1>{flow.metadata.name || 'Untitled flow'}</h1>
      <p class="text-lighter text-xs">
        <code>{flow.metadata.id || '…'}</code>
        · {flow.metadata.execution_mode === 'dag' ? 'dependency graph' : 'one after another'}
        {#if flow.metadata.prefix}· {flow.metadata.prefix}{/if}
      </p>
    </div>

    <div class="bar-actions hstack gap-2">
      {#if errors.length > 0}
        <button
          type="button"
          class="badge danger"
          aria-expanded={showProblems}
          onclick={() => (showProblems = !showProblems)}
        >
          <IconAlertTriangle size={14} />
          {errors.length} to fix
        </button>
      {/if}
      <button type="button" data-variant="secondary" onclick={onCancel}>
        {readonly ? 'Back' : 'Cancel'}
      </button>
      {#if !readonly}
        <button type="button" onclick={onSave} disabled={saving || errors.length > 0}>
          <span class="hstack gap-2 justify-center" aria-busy={saving} data-spinner="small">
            {#if saving}
              {mode === 'create' ? 'Creating...' : 'Saving...'}
            {:else}
              {mode === 'create' ? 'Create flow' : 'Save changes'}
            {/if}
          </span>
        </button>
      {/if}
    </div>
  </header>

  <div class="body">
    <nav class="rail">
      <div role="tablist" aria-label="Flow sections" aria-orientation="vertical">
        {#each SECTIONS as item (item.key)}
          {@const locked = mode === 'create' && item.key === 'secrets'}
          <button
            type="button"
            role="tab"
            aria-selected={section === item.key}
            disabled={locked}
            title={locked ? 'Available once the flow exists' : undefined}
            onclick={() => (section = item.key)}
          >
            <item.icon size={16} />
            <span class="rail-label">{item.label}</span>
            {#if errorCounts[item.key]}
              <span class="badge danger small" title="{errorCounts[item.key]} problem(s)">!</span>
            {:else if counts[item.key] !== undefined}
              <span class="text-lighter text-xs">{counts[item.key] || '—'}</span>
            {/if}
          </button>
        {/each}
      </div>
    </nav>

    <section class="content" role="tabpanel">
      {#if section === 'general'}
        <GeneralSection bind:flow {namespace} nameLocked={mode === 'edit'} disabled={readonly} />
      {:else if section === 'inputs'}
        <InputsSection bind:inputs={flow.inputs} actions={flow.actions} disabled={readonly} />
      {:else if section === 'actions'}
        <ActionsSection
          bind:flow
          {namespace}
          {availableExecutors}
          bind:executorConfigs
          bind:selectedId={selectedActionId}
          disabled={readonly}
        />
      {:else if section === 'notify'}
        <div class="pane-scroll">
          <div class="pane-inner card">
            <FlowNotifications
              bind:notifications={flow.notifications}
              {availableMessengers}
              {messengerConfigs}
              disabled={readonly}
            />
          </div>
        </div>
      {:else}
        <div class="pane-scroll">
          <div class="pane-inner card">
            <SecretsTab {namespace} {flowId} disabled={readonly} />
          </div>
        </div>
      {/if}

      {#if problemsOpen}
        <ProblemStrip {problems} onSelect={showProblem} onClose={() => (showProblems = false)} />
      {/if}
    </section>
  </div>
</div>

<style>
  .builder {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    background: var(--card);
  }
  /* The workspace fills the frame, so the shell's footer is dead weight here. */
  .builder + :global(.app-footer) {
    display: none;
  }

  .bar {
    flex-shrink: 0;
    padding: var(--space-3) var(--space-6);
    background: var(--card);
    border-bottom: 1px solid var(--border);
  }
  .title {
    min-width: 0;
  }
  .title h1 {
    margin: 0;
    font-size: var(--text-4);
    line-height: 1.2;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .title p {
    margin: 2px 0 0;
  }
  .bar-actions {
    margin-inline-start: auto;
    flex-wrap: nowrap;
  }

  .body {
    flex: 1;
    min-height: 0;
    display: flex;
  }

  .rail {
    flex-shrink: 0;
    padding: var(--space-4) var(--space-3);
    background: var(--card);
    border-inline-end: 1px solid var(--border);
    overflow: hidden auto;
  }
  /* Oat's tab strip, stood up and flattened to match the app sidebar's nav:
     no darker tray, and the active item in --faint like Sidebar.svelte. */
  .rail [role='tablist'] {
    flex-direction: column;
    align-items: stretch;
    width: 12.5rem;
    padding: 0;
    background: transparent;
  }
  .rail [role='tab'][aria-selected='true'] {
    background: var(--faint);
    box-shadow: none;
  }
  .rail [role='tab'] {
    justify-content: flex-start;
    gap: var(--space-3);
  }
  .rail [role='tab']:disabled {
    color: var(--faint-foreground);
    cursor: not-allowed;
  }
  .rail-label {
    flex: 1;
    min-width: 0;
    text-align: start;
  }

  .content {
    flex: 1;
    min-width: 0;
    min-height: 0;
    display: flex;
    flex-direction: column;
    padding: 0;
  }

  @media (max-width: 1000px) {
    .body {
      flex-direction: column;
    }
    .rail {
      border-inline-end: 0;
      border-bottom: 1px solid var(--border);
      padding: var(--space-2) var(--space-3);
    }
    .rail [role='tablist'] {
      flex-direction: row;
      width: auto;
      overflow-x: auto;
    }
  }
</style>
