<script lang="ts">
  import OatSelect from '$lib/components/shared/OatSelect.svelte';
  import CodeEditor from '$lib/components/shared/CodeEditor.svelte';
  import KeyValueEditor from '$lib/components/shared/KeyValueEditor.svelte';

  let {
    schema,
    values = $bindable(),
    idPrefix,
    codeHeight = '260px',
    disabled = false
  }: {
    schema: any;
    values: Record<string, any>;
    idPrefix: string;
    codeHeight?: string;
    disabled?: boolean;
  } = $props();

  const properties = $derived(Object.entries(schema?.properties ?? {}) as Array<[string, any]>);
  const isRequired = (key: string) => schema?.required?.includes(key) ?? false;
</script>

{#each properties as [key, property] (key)}
  {@const id = `${idPrefix}-${key}`}
  {@const title = property.title || key}
  {@const placeholder = property.placeholder || property.default || ''}

  <div data-field>
    {#if property.type === 'checkbox'}
      <label class="hstack gap-2">
        <input type="checkbox" role="switch" {id} bind:checked={values[key]} {disabled} />
        <span>{title}</span>
      </label>
    {:else}
      <label for={id}>{title}{#if isRequired(key)}<span class="req">*</span>{/if}</label>

      {#if property.enum}
        <OatSelect
          bind:value={values[key]}
          options={property.enum.map((o: string) => ({ value: o, label: o }))}
          placeholder="Select..."
          {id}
          {disabled}
        />
      {:else if property.type === 'number' || property.type === 'integer'}
        <input
          type="number"
          {id}
          {disabled}
          {placeholder}
          step={property.type === 'integer' ? '1' : 'any'}
          min={property.minimum}
          max={property.maximum}
          bind:value={values[key]}
        />
      {:else if property.widget === 'codeeditor'}
        <CodeEditor
          value={values[key] || ''}
          height={codeHeight}
          readonly={disabled}
          onchange={(value) => (values[key] = value)}
        />
      {:else if property.widget === 'keyvalue'}
        <KeyValueEditor
          initialValue={values[key]}
          valuePlaceholder={placeholder || 'value'}
          onchange={(json) => (values[key] = json)}
          {disabled}
        />
      {:else if property.format === 'textarea' || property.type === 'object' || property.type === 'array'}
        <textarea
          {id}
          {disabled}
          rows="4"
          placeholder={placeholder || (property.type === 'string' ? 'Multi-line text' : 'JSON')}
          bind:value={values[key]}
        ></textarea>
      {:else}
        <input
          {id}
          {disabled}
          {placeholder}
          type={property.format === 'password' ? 'password' : 'text'}
          bind:value={values[key]}
        />
      {/if}
    {/if}

    {#if property.description}<span data-hint>{property.description}</span>{/if}
  </div>
{/each}

<style>
  .req {
    margin-inline-start: var(--space-1);
  }
</style>
