<script lang="ts">
  import { get } from 'svelte/store';
  import { IconPlus, IconX } from '@tabler/icons-svelte';
  import FlowGroupSelector from '$lib/components/flow-create/FlowGroupSelector.svelte';
  import { appInfo } from '$lib/stores/auth';
  import { createSlug, isValidCronExpression } from '$lib/utils';
  import { getTimezones } from '$lib/utils/timezone';
  import type { BuilderFlow } from '$lib/types';

  let {
    flow = $bindable(),
    namespace,
    nameLocked = false,
    disabled = false
  }: {
    flow: BuilderFlow;
    namespace: string;
    nameLocked?: boolean;
    disabled?: boolean;
  } = $props();

  const timezones = getTimezones();

  // A schedule supplies no input values, so every input has to stand on its own default.
  const schedulable = $derived(
    !flow.inputs.some((input) => input.type === 'file' || !input.default?.trim())
  );

  function updateName(value: string) {
    flow.metadata.name = value;
    flow.metadata.id = createSlug(value);
  }

  function addSchedule() {
    flow.metadata.schedules.push({
      name: '',
      cron: '',
      timezone: get(appInfo)?.default_timezone ?? 'UTC'
    });
  }
</script>

<div class="pane-scroll">
  <div class="pane-inner vstack gap-6">
    <section class="card">
      <h2 class="mb-4">Identity</h2>
      <div class="vstack gap-4">
        <div data-field>
          <label for="flow-name">Name <span class="req">*</span></label>
          <input
            id="flow-name"
            type="text"
            value={flow.metadata.name}
            oninput={(e) => updateName(e.currentTarget.value)}
            disabled={nameLocked || disabled}
            placeholder="Deploy kite"
          />
          <span data-hint>
            {#if nameLocked}
              Fixed after creation. The flow id is <code>{flow.metadata.id}</code>.
            {:else}
              The flow id <code>{flow.metadata.id || '…'}</code> comes from the name and cannot be
              changed later.
            {/if}
          </span>
        </div>

        <div data-field>
          <label for="flow-description">Description</label>
          <textarea
            id="flow-description"
            rows="2"
            {disabled}
            bind:value={flow.metadata.description}
            placeholder="What does this flow do?"
          ></textarea>
        </div>

        <FlowGroupSelector {namespace} bind:value={flow.metadata.prefix} />
      </div>
    </section>

    <section class="card">
      <h2 class="mb-4">Execution</h2>
      <div class="vstack gap-4">
        <div class="row" role="radiogroup" aria-label="Execution order">
          <label class="col-6 choice">
            <input
              type="radio"
              name="execution-mode"
              value="sequential"
              checked={flow.metadata.execution_mode !== 'dag'}
              {disabled}
              onchange={() => (flow.metadata.execution_mode = 'sequential')}
            />
            <span class="font-medium">One after another</span>
            <span class="sub text-xs text-lighter">In the order they are listed.</span>
          </label>
          <label class="col-6 choice">
            <input
              type="radio"
              name="execution-mode"
              value="dag"
              checked={flow.metadata.execution_mode === 'dag'}
              {disabled}
              onchange={() => (flow.metadata.execution_mode = 'dag')}
            />
            <span class="font-medium">Dependency graph</span>
            <span class="sub text-xs text-lighter">
              Each action starts when the ones it waits for finish.
            </span>
          </label>
        </div>

        {#if flow.metadata.execution_mode === 'dag'}
          <div data-field>
            <label for="max-parallel-actions">Maximum parallel actions</label>
            <input
              id="max-parallel-actions"
              type="number"
              min="0"
              max="64"
              {disabled}
              bind:value={flow.metadata.max_parallel}
            />
            <span data-hint><code>0</code> means no limit.</span>
          </div>
        {/if}

        <div data-field>
          <label for="max-retries">Retries</label>
          <input
            id="max-retries"
            type="number"
            min="0"
            max="10"
            {disabled}
            bind:value={flow.metadata.max_retries}
          />
          <span data-hint>Exponential backoff, 15s up to 5 min. <code>0</code> disables it.</span>
        </div>

        <label class="hstack gap-2">
          <input
            type="checkbox"
            role="switch"
            {disabled}
            bind:checked={flow.metadata.allow_overlap}
          />
          <span>Allow overlapping executions</span>
        </label>
      </div>
    </section>

    <section class="card">
      <h2 class="mb-4">Scheduling</h2>

      {#if !schedulable}
        <div role="alert" data-variant="warning" class="mb-4">
          <div>
            <strong>Some inputs have no default.</strong>
            A scheduled run cannot supply them.
          </div>
        </div>
      {/if}

      <div class="vstack gap-4">
        <div>
          <div class="hstack justify-between mb-2">
            <label for="schedules">Cron schedules <span class="text-lighter">(optional)</span></label>
            {#if !disabled}
              <button type="button" data-variant="secondary" class="ghost small" onclick={addSchedule}>
                <IconPlus size={14} /> Add schedule
              </button>
            {/if}
          </div>

          <div class="vstack gap-4" id="schedules">
            {#each flow.metadata.schedules as schedule, i (i)}
              {@const invalid = schedule.cron !== '' && !isValidCronExpression(schedule.cron)}
              <div class="card p-4">
                <div class="hstack gap-2 items-start nowrap">
                  <div class="row flex-1">
                    <div class="col-4" data-field>
                      <label for="schedule-name-{i}">Name</label>
                      <input
                        id="schedule-name-{i}"
                        type="text"
                        maxlength="150"
                        {disabled}
                        bind:value={schedule.name}
                        placeholder="Nightly"
                      />
                    </div>
                    <div class="col-4" data-field={invalid ? 'error' : ''}>
                      <label for="schedule-cron-{i}">Cron expression</label>
                      <input
                        id="schedule-cron-{i}"
                        type="text"
                        class="font-mono"
                        {disabled}
                        bind:value={schedule.cron}
                        aria-invalid={invalid ? 'true' : undefined}
                        placeholder="0 2 * * *"
                      />
                      <span class="error">minute hour day month weekday</span>
                    </div>
                    <div class="col-4" data-field>
                      <label for="schedule-tz-{i}">Timezone</label>
                      <input
                        id="schedule-tz-{i}"
                        type="text"
                        list="timezone-list"
                        {disabled}
                        bind:value={schedule.timezone}
                        placeholder="UTC"
                      />
                    </div>
                  </div>
                  {#if !disabled}
                    <button
                      type="button"
                      data-variant="danger"
                      class="ghost icon small"
                      aria-label="Remove schedule"
                      onclick={() => flow.metadata.schedules.splice(i, 1)}
                    >
                      <IconX size={16} />
                    </button>
                  {/if}
                </div>
              </div>
            {/each}

            {#if flow.metadata.schedules.length === 0}
              <p class="text-lighter text-sm">
                None. Examples: <code>0 2 * * *</code> daily at 2 AM ·
                <code>0 */6 * * *</code> every 6 hours · <code>30 9 * * 1-5</code> weekdays at 9:30.
              </p>
            {/if}
          </div>

          {#if flow.metadata.schedules.length > 0}
            <datalist id="timezone-list">
              {#each timezones as tz (tz.tzCode)}
                <option value={tz.tzCode}>{tz.label}</option>
              {/each}
            </datalist>
          {/if}
        </div>

        <label class="hstack gap-2">
          <input
            type="checkbox"
            role="switch"
            {disabled}
            bind:checked={flow.metadata.user_schedulable}
          />
          <span>Let users add their own schedules</span>
        </label>
      </div>
    </section>
  </div>
</div>

<style>
  h2 {
    font-size: var(--text-5);
  }

  /* A radio that reads as a card, for the one choice that reshapes the editor. */
  .choice {
    display: grid;
    grid-template-columns: auto 1fr;
    column-gap: var(--space-3);
    align-items: start;
    padding: var(--space-4);
    border: 1px solid var(--border);
    border-radius: var(--radius-medium);
    cursor: pointer;
  }
  .choice:hover {
    border-color: var(--primary);
  }
  .choice:has(input:checked) {
    border-color: var(--primary);
    background: var(--accent);
  }
  .choice .sub {
    grid-column: 2;
  }
</style>
