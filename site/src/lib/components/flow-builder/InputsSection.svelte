<script lang="ts">
  import IconPlus from '@tabler/icons-svelte/icons/plus';
  import type { BuilderAction, BuilderInput, FlowInputReq } from '$lib/types';
  import { newInput } from '$lib/utils/flowBuilder';
  import InputCard from './InputCard.svelte';
  import { INPUT_TYPES, inputReferences } from './inputTypes';

  let {
    inputs = $bindable(),
    actions,
    disabled = false
  }: {
    inputs: BuilderInput[];
    actions: BuilderAction[];
    disabled?: boolean;
  } = $props();

  const quickAdd: Array<FlowInputReq['type']> = ['string', 'select', 'checkbox', 'file', 'node'];
  let openIndex = $state(-1);

  const references = $derived(inputReferences(actions));

  function add(type: FlowInputReq['type'] = 'string') {
    inputs.push(newInput(type));
    openIndex = inputs.length - 1;
  }
</script>

<div class="pane-scroll">
  <div class="pane-inner">
    <div class="hstack justify-between items-start mb-6">
      <div>
        <h2>Inputs</h2>
        <p class="text-sm text-lighter">Read by actions as <code>{'{{ inputs.name }}'}</code>.</p>
      </div>
      {#if !disabled}
        <button type="button" class="small" onclick={() => add()}>
          <IconPlus size={14} /> Add input
        </button>
      {/if}
    </div>

    {#if inputs.length === 0}
      <div class="card align-center">
        <h3>No inputs yet</h3>
        <p class="text-lighter">Without inputs this flow runs the same way every time.</p>
        {#if !disabled}
          <div class="hstack gap-2 justify-center mt-4">
            {#each quickAdd as type (type)}
              {@const meta = INPUT_TYPES.find((t) => t.value === type)!}
              <button type="button" class="outline small" onclick={() => add(type)}>
                <IconPlus size={14} />
                {meta.label}
              </button>
            {/each}
          </div>
        {/if}
      </div>
    {:else}
      {#each inputs as _, i (i)}
        <InputCard
          bind:input={inputs[i]}
          {references}
          index={i}
          {disabled}
          open={openIndex === i}
          onToggle={(isOpen) => {
            if (isOpen) openIndex = i;
            else if (openIndex === i) openIndex = -1;
          }}
          onRemove={() => {
            inputs.splice(i, 1);
            openIndex = -1;
          }}
        />
      {/each}
    {/if}
  </div>
</div>

<style>
  h2,
  p {
    margin: 0;
  }
</style>
