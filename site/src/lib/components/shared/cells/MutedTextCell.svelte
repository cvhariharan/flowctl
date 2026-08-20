<script lang="ts">
  let {
    row,
    value,
    format,
    mono = false,
    lighter = false,
    plain = false,
    truncate = 0,
    maxWidth = '20rem',
    class: className = '',
    tooltip
  }: {
    row: any;
    value: any;
    format?: (value: any, row: any) => string;
    mono?: boolean;
    lighter?: boolean;
    plain?: boolean;
    truncate?: number;
    maxWidth?: string;
    class?: string;
    tooltip?: string | ((value: any, row: any) => string | undefined);
  } = $props();

  const formatted = $derived(format ? format(value, row) : (value ?? ''));
  const isTruncated = $derived(truncate > 0 && typeof formatted === 'string' && formatted.length > truncate);
  const tooltipText = $derived(typeof tooltip === 'function' ? tooltip(value, row) : tooltip);
  const toneClass = $derived(plain ? '' : lighter ? 'text-lighter' : 'cell-muted');
</script>

{#if truncate > 0}
  <span
    class="{toneClass} {mono ? 'font-mono' : ''} {className}"
    style="max-width:{maxWidth};overflow:hidden;text-overflow:ellipsis;white-space:nowrap;display:block"
    title={tooltipText || (isTruncated ? formatted : undefined)}
  >{formatted}</span>
{:else}
  <span
    class="{toneClass} {mono ? 'font-mono' : ''} {className}"
    title={tooltipText}
  >{formatted}</span>
{/if}
