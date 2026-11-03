<script lang="ts">
  import type { FlowDiffLine } from '$lib/types';

  let { lines }: { lines: FlowDiffLine[] } = $props();
  let expanded = $state<Set<number>>(new Set());
  const groups = $derived.by(() => {
    const result: { start: number; lines: FlowDiffLine[]; equal: boolean }[] = [];
    for (let i = 0; i < lines.length; i++) {
      const equal = lines[i].op === 'equal';
      const last = result[result.length - 1];
      if (last?.equal === equal) last.lines.push(lines[i]);
      else result.push({ start: i, lines: [lines[i]], equal });
    }
    return result;
  });
  $effect(() => { lines; expanded = new Set(); });
</script>

{#snippet row(line: FlowDiffLine)}
  <tr class:insert={line.op === 'insert'} class:delete={line.op === 'delete'}>
    <td class="gutter">{line.old ?? ''}</td>
    <td class="gutter">{line.new ?? ''}</td>
    <td class="marker">{line.op === 'insert' ? '+' : line.op === 'delete' ? '-' : ' '}</td>
    <td class="source">{line.text}</td>
  </tr>
{/snippet}

<div class="diff-scroll" role="region" aria-label="Flow source diff">
  <table>
    <tbody>
      {#each groups as group (group.start)}
        {#if group.equal && group.lines.length > 6 && !expanded.has(group.start)}
          {#each group.lines.slice(0, 3) as line}{@render row(line)}{/each}
          <tr><td colspan="4"><button class="outline small" onclick={() => expanded = new Set([...expanded, group.start])}>Show {group.lines.length - 6} unchanged lines</button></td></tr>
          {#each group.lines.slice(-3) as line}{@render row(line)}{/each}
        {:else}
          {#each group.lines as line}{@render row(line)}{/each}
        {/if}
      {/each}
    </tbody>
  </table>
</div>

<style>
  .diff-scroll { max-width: 100%; min-width: 0; overflow-x: auto; }
  table { width: 100%; border-collapse: collapse; font-family: monospace; font-size: var(--text-7); }
  td { padding: 0 var(--space-2); border: 0; }
  .gutter { width: 1%; text-align: right; color: var(--muted-foreground); user-select: none; }
  .marker { width: 1%; user-select: none; }
  .source { white-space: pre; }
  .insert { background: color-mix(in srgb, var(--success) 15%, transparent); }
  .delete { background: color-mix(in srgb, var(--danger) 15%, transparent); }
</style>
