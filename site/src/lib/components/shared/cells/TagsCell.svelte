<script lang="ts">
  let { row, value }: { row: any; value: any } = $props();

  const tags: string[] = $derived(Array.isArray(value) ? value : []);
</script>

{#if tags.length > 0}
  <div class="hstack gap-1" style="flex-wrap:wrap">
    {#each tags as tag}
      <span class="badge primary">{tag}</span>
    {/each}
  </div>
{:else}
  <span class="cell-muted text-xs">No tags</span>
{/if}
