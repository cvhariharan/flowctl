<script lang="ts">
  let {
    row,
    value,
    variant = 'primary',
    label,
    tooltip
  }: {
    row: any;
    value: any;
    variant?: string | ((row: any) => string);
    label?: string | ((row: any) => string);
    tooltip?: string | ((row: any) => string | undefined);
  } = $props();

  const resolvedVariant = $derived(typeof variant === 'function' ? variant(row) : variant);
  const resolvedLabel = $derived(typeof label === 'function' ? label(row) : (label ?? value));
  const resolvedTooltip = $derived(typeof tooltip === 'function' ? tooltip(row) : tooltip);
</script>

<span class="badge {resolvedVariant}" title={resolvedTooltip}>{resolvedLabel}</span>
