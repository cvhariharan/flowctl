<script lang="ts">
  import IconBolt from '@tabler/icons-svelte/icons/bolt';

  let { value }: { value: string } = $props();
</script>

<div class="hstack">
  <div class="icon-box">
    <IconBolt size={16} />
  </div>
  <div class="min-w-0 flex-1">
    <code class="exec-code text-lighter">{value}</code>
  </div>
</div>

<style>
  .exec-code {
    font-size: var(--text-8);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    display: block;
  }
</style>
