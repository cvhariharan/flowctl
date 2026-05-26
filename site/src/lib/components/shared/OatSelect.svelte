<script lang="ts">
  import { IconChevronDown } from '@tabler/icons-svelte';

  type Option = { label: string; value: string };

  let {
    value = $bindable(),
    options = [],
    placeholder = 'Select...',
    disabled = false,
    required = false,
    id,
    name,
    onchange
  }: {
    value?: string;
    options: Option[];
    placeholder?: string;
    disabled?: boolean;
    required?: boolean;
    id?: string;
    name?: string;
    onchange?: () => void;
  } = $props();

  const popoverId = id ? `oat-sel-${id}` : `oat-sel-${Math.random().toString(36).slice(2, 7)}`;
  let selectedLabel = $derived(options.find(o => o.value === value)?.label || '');

  function select(opt: Option) {
    value = opt.value;
    document.getElementById(popoverId)?.hidePopover();
    onchange?.();
  }
</script>

<ot-dropdown style="display: block; margin-block-start: var(--space-1)">
  <button
    type="button"
    popovertarget={popoverId}
    class="outline w-100 hstack justify-between"
    {disabled}
  >
    <span class={!value ? 'text-light' : ''}>{selectedLabel || placeholder}</span>
    <IconChevronDown size={16} aria-hidden="true" />
  </button>
  <div popover id={popoverId} style="max-height: 16rem; overflow-y: auto">
    {#each options as opt}
      <button
        type="button"
        role="menuitem"
        aria-selected={opt.value === value}
        onclick={() => select(opt)}
      >
        {opt.label}
      </button>
    {/each}
  </div>
</ot-dropdown>

{#if required || name}
  <select bind:value {name} {required} {disabled} tabindex="-1" aria-hidden="true" style="position: absolute; opacity: 0; height: 0; pointer-events: none; overflow: hidden">
    <option value="">{placeholder}</option>
    {#each options as opt}
      <option value={opt.value}>{opt.label}</option>
    {/each}
  </select>
{/if}
