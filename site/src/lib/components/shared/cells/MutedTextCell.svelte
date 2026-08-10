<script lang="ts">
  let {
    row,
    value,
    format,
    mono = false,
    lighter = false,
    truncate = 0,
    tooltip
  }: {
    row: any;
    value: any;
    format?: (value: any, row: any) => string;
    mono?: boolean;
    lighter?: boolean;
    truncate?: number;
    tooltip?: string | ((value: any, row: any) => string | undefined);
  } = $props();

  const formatted = $derived(format ? format(value, row) : (value ?? ''));
  const isTruncated = $derived(truncate > 0 && typeof formatted === 'string' && formatted.length > truncate);
  const tooltipText = $derived(typeof tooltip === 'function' ? tooltip(value, row) : tooltip);
</script>

{#if truncate > 0}
  <span
    class="{lighter ? 'text-lighter' : 'cell-muted'} {mono ? 'font-mono' : ''}"
    style="max-width:20rem;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;display:block"
    title={tooltipText || (isTruncated ? formatted : undefined)}
  >{formatted}</span>
{:else}
  <span
    class="{lighter ? 'text-lighter' : 'cell-muted'} {mono ? 'font-mono' : ''}"
    title={tooltipText}
  >{formatted}</span>
{/if}
