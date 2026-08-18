<script lang="ts">
  import IconUser from '@tabler/icons-svelte/icons/user';

  let { value }: { row: any; value: any } = $props();

  let displayName = $derived(value ? value.replace(/<.*>$/, '').trim() : 'System');
  let tooltip = $derived(value?.match(/<(.+)>/)?.[1] ?? '');
</script>

<div class="hstack gap-2" title={tooltip}>
  <div class="icon-box" style="width:2rem;height:2rem;border-radius:50%">
    <IconUser size={16} />
  </div>
  <span class="text-sm">{displayName}</span>
</div>
