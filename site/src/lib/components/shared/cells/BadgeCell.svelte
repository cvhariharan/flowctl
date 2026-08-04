<script lang="ts">
  let {
    row,
    value,
    variant = 'primary',
    label
  }: {
    row: any;
    value: any;
    variant?: string | ((row: any) => string);
    label?: string | ((row: any) => string);
  } = $props();

  const resolvedVariant = $derived(typeof variant === 'function' ? variant(row) : variant);
  const resolvedLabel = $derived(typeof label === 'function' ? label(row) : (label ?? value));
</script>

<span class="badge {resolvedVariant}">{resolvedLabel}</span>
