<script lang="ts">
  import { tick } from 'svelte';
  import IconGripVertical from '@tabler/icons-svelte/icons/grip-vertical';
  import IconPlus from '@tabler/icons-svelte/icons/plus';
  import IconX from '@tabler/icons-svelte/icons/x';
  import KeyValueEditor from '$lib/components/shared/KeyValueEditor.svelte';
  import OatSelect from '$lib/components/shared/OatSelect.svelte';
  import type { BuilderInput } from '$lib/types';
  import {
    INPUT_TYPES,
    applyInputType,
    inputIcon,
    referencesFor,
    type InputReferences
  } from './inputTypes';

  let {
    input = $bindable(),
    references,
    index,
    open = false,
    disabled = false,
    onToggle,
    onRemove
  }: {
    input: BuilderInput;
    references: InputReferences;
    index: number;
    open?: boolean;
    disabled?: boolean;
    onToggle: (open: boolean) => void;
    onRemove: () => void;
  } = $props();

  const TypeIcon = $derived(inputIcon(input.type));
  const usedBy = $derived(referencesFor(input, references));

  let dragFrom = $state(-1);
  let dragOver = $state(-1);

  function toggleRemote(useRemote: boolean) {
    input.useRemoteOptions = useRemote;
    if (useRemote) {
      input.remote_options ??= { url: '', method: 'GET', headers: {} };
    }
  }

  async function moveOption(from: number, to: number) {
    if (from < 0 || to < 0 || from === to || to >= input.options.length) return;

    const [moved] = input.options.splice(from, 1);
    input.options.splice(to, 0, moved);

    await tick();
    document.getElementById(`input-${index}-opt-${to}-grip`)?.focus();
  }

  function dropOption(to: number) {
    moveOption(dragFrom, to);
    dragFrom = -1;
    dragOver = -1;
  }
</script>

<details {open} ontoggle={(e) => onToggle(e.currentTarget.open)}>
  <summary>
    <span class="icon-box"><TypeIcon size={16} /></span>
    <span class="flex-1 min-w-0">
      <span class="title font-medium">{input.label || input.name || `Input ${index + 1}`}</span>
      <span class="hstack gap-2 text-xs text-lighter">
        {#if input.name}<code>{input.name}</code>{/if}
        <span class="badge outline">{input.type}</span>
        {#if input.required}<span class="badge warning">required</span>{/if}
      </span>
    </span>
    {#if !disabled}
      <button
        type="button"
        data-variant="danger"
        class="ghost icon small"
        aria-label="Remove input"
        onclick={(e) => {
          e.preventDefault();
          onRemove();
        }}
      >
        <IconX size={16} />
      </button>
    {/if}
  </summary>

  <div>
    <div data-field>
      <label for="input-{index}-type">Type <span class="req">*</span></label>
      <div class="types" role="radiogroup" aria-label="Input type" id="input-{index}-type">
        {#each INPUT_TYPES as type (type.value)}
          <label class:on={input.type === type.value}>
            <input
              type="radio"
              name="input-type-{index}"
              value={type.value}
              checked={input.type === type.value}
              {disabled}
              onchange={() => applyInputType(input, type.value)}
            />
            <type.icon size={16} />
            {type.label}
          </label>
        {/each}
      </div>
    </div>

    <div class="row">
      <div class="col-6" data-field>
        <label for="input-{index}-name">Name <span class="req">*</span></label>
        <input
          id="input-{index}-name"
          type="text"
          class="font-mono"
          {disabled}
          value={input.name}
          oninput={(e) => (input.name = e.currentTarget.value.replace(/[^a-zA-Z0-9_]/g, ''))}
          placeholder="deploy_tag"
        />
        {#if input.name}
          <span data-hint>
            {usedBy.length > 0 ? `Used by ${usedBy.join(', ')}` : 'Not referenced by any action'}
          </span>
        {/if}
      </div>
      <div class="col-6" data-field>
        <label for="input-{index}-label">Label</label>
        <input id="input-{index}-label" type="text" {disabled} bind:value={input.label} />
      </div>
    </div>

    <div data-field>
      <label for="input-{index}-description">Description</label>
      <input
        id="input-{index}-description"
        type="text"
        {disabled}
        bind:value={input.description}
        placeholder="Help text shown under the field"
      />
    </div>

    {#if input.type === 'select'}
      <div class="sub mb-4">
        <div class="hstack justify-between mb-4">
          <span class="font-medium">Options</span>
          <menu class="buttons">
            <li>
              <button
                type="button"
                {disabled}
                data-variant={input.useRemoteOptions ? 'secondary' : undefined}
                onclick={() => toggleRemote(false)}
              >
                Static list
              </button>
            </li>
            <li>
              <button
                type="button"
                {disabled}
                data-variant={input.useRemoteOptions ? undefined : 'secondary'}
                onclick={() => toggleRemote(true)}
              >
                Fetched from API
              </button>
            </li>
          </menu>
        </div>

        {#if input.useRemoteOptions && input.remote_options}
          <div data-field>
            <label for="input-{index}-url">URL <span class="req">*</span></label>
            <fieldset class="group">
              <select bind:value={input.remote_options.method} {disabled} aria-label="Method">
                <option>GET</option>
                <option>POST</option>
              </select>
              <input
                id="input-{index}-url"
                type="url"
                class="font-mono"
                {disabled}
                bind:value={input.remote_options.url}
                placeholder="https://api.example.com/options"
              />
            </fieldset>
          </div>

          <div data-field>
            <span class="label">Headers</span>
            <KeyValueEditor
              initialValue={JSON.stringify(input.remote_options.headers ?? {})}
              keyPlaceholder="Authorization"
              valuePlaceholder={'{{ secrets.API_KEY }}'}
              onchange={(json) => {
                if (input.remote_options) input.remote_options.headers = JSON.parse(json);
              }}
              {disabled}
            />
            <span data-hint>The response should be a JSON array of strings.</span>
          </div>
        {:else}
          <div class="vstack gap-2">
            {#each input.options as _, oi (oi)}
              <div
                class="hstack gap-2 nowrap option"
                class:over={dragOver === oi && dragFrom !== oi}
                role="presentation"
                ondragover={(e) => {
                  if (dragFrom < 0) return;
                  e.preventDefault();
                  dragOver = oi;
                }}
                ondragleave={() => {
                  if (dragOver === oi) dragOver = -1;
                }}
                ondrop={(e) => {
                  e.preventDefault();
                  dropOption(oi);
                }}
              >
                <button
                  type="button"
                  id="input-{index}-opt-{oi}-grip"
                  class="ghost icon small grip"
                  draggable={!disabled}
                  {disabled}
                  aria-label="Move option {oi + 1}. Use the up and down arrow keys to reorder."
                  ondragstart={(e) => {
                    dragFrom = oi;
                    e.dataTransfer?.setData('text/plain', String(oi));
                  }}
                  ondragend={() => {
                    dragFrom = -1;
                    dragOver = -1;
                  }}
                  onkeydown={(e) => {
                    if (e.key !== 'ArrowUp' && e.key !== 'ArrowDown') return;
                    e.preventDefault();
                    moveOption(oi, e.key === 'ArrowUp' ? oi - 1 : oi + 1);
                  }}
                >
                  <IconGripVertical size={14} />
                </button>
                <input
                  type="text"
                  class="flex-1"
                  {disabled}
                  bind:value={input.options[oi]}
                  aria-label="Option {oi + 1}"
                />
                <button
                  type="button"
                  data-variant="danger"
                  class="ghost icon small"
                  {disabled}
                  aria-label="Remove option"
                  onclick={() => input.options.splice(oi, 1)}
                >
                  <IconX size={14} />
                </button>
              </div>
            {/each}
          </div>
          {#if !disabled}
            <button type="button" class="ghost small mt-2" onclick={() => input.options.push('')}>
              <IconPlus size={14} /> Add option
            </button>
          {/if}
          {#if input.options.length > 0}
            <div class="hstack gap-2 items-center mt-4">
              <label for="input-{index}-default-option" class="text-sm">Default</label>
              <OatSelect
                bind:value={input.default}
                options={[
                  { value: '', label: 'No default' },
                  ...input.options.filter(Boolean).map((o) => ({ value: o, label: o }))
                ]}
                id="input-{index}-default-option"
                {disabled}
              />
            </div>
          {/if}
        {/if}
      </div>
    {/if}

    {#if input.type === 'file'}
      <div class="sub mb-4">
        <div data-field>
          <label for="input-{index}-size">Maximum size (MB)</label>
          <input
            id="input-{index}-size"
            type="number"
            min="1"
            {disabled}
            bind:value={input.maxFileSizeMB}
            placeholder="Server default"
          />
          <span data-hint>A flow with a file input cannot be scheduled.</span>
        </div>
      </div>
    {/if}

    {#if input.type === 'node'}
      <div class="sub mb-4">
        <label class="hstack gap-2">
          <input type="checkbox" role="switch" {disabled} bind:checked={input.multiple} />
          <span>Let the operator pick more than one node</span>
        </label>
        <span data-hint>Only one input of type node is allowed per flow.</span>
      </div>
    {/if}

    <label class="hstack gap-2 mb-4">
      <input type="checkbox" role="switch" {disabled} bind:checked={input.required} />
      <span>Required</span>
    </label>

    <div class="row">
      {#if input.type !== 'file' && input.type !== 'select'}
        <div class="col-6" data-field>
          <label for="input-{index}-default">Default value</label>
          <input id="input-{index}-default" type="text" {disabled} bind:value={input.default} />
          <span data-hint>Scheduled runs need one.</span>
        </div>
      {/if}
      <div class="col-6" data-field>
        <label for="input-{index}-validation">Validation expression</label>
        <input
          id="input-{index}-validation"
          type="text"
          class="font-mono"
          {disabled}
          bind:value={input.validation}
          placeholder="len({input.name || 'value'}) > 3"
        />
      </div>
    </div>
  </div>
</details>

<style>
  .title {
    display: block;
  }

  /* The eight types, visible at once. Oat has no segmented picker. */
  .types {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: var(--space-2);
  }
  .types label {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: var(--space-2);
    padding: var(--space-2);
    border: 1px solid var(--border);
    border-radius: var(--radius-medium);
    font-size: var(--text-8);
    font-weight: var(--font-medium);
    color: var(--muted-foreground);
    cursor: pointer;
  }
  .types label:hover {
    background: var(--faint);
    color: var(--foreground);
  }
  .types label.on {
    background: var(--accent);
    border-color: var(--primary);
    color: var(--primary);
  }
  .types input {
    display: none;
  }

  .option {
    border-block-start: 2px solid transparent;
  }
  .option.over {
    border-block-start-color: var(--primary);
  }
  .grip {
    cursor: grab;
    color: var(--muted-foreground);
  }
  .grip:active {
    cursor: grabbing;
  }

  /* Fields that only exist for the selected type. */
  .sub {
    padding: var(--space-4);
    background: var(--faint);
    border-radius: var(--radius-medium);
  }
  .sub > :last-child {
    margin-block-end: 0;
  }

  @media (max-width: 900px) {
    .types {
      grid-template-columns: repeat(2, 1fr);
    }
  }
</style>
