<script lang="ts">
  let {
    results
  }: {
    results: Record<string, any>
  } = $props();
</script>

{#if Object.keys(results).length > 0}
  <div class="overflow-hidden rounded-lg border border-gray-200">
    <table class="min-w-full divide-y divide-gray-200">
      <thead class="bg-gray-50">
        <tr>
          <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Variable</th>
          <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Value</th>
        </tr>
      </thead>
      <tbody class="bg-white divide-y divide-gray-200">
        {#each Object.entries(results) as [key, value]}
          <tr class="hover:bg-gray-50 transition-colors">
            <td class="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900 font-mono">{key}</td>
            <td class="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900 font-mono">{value}</td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
{/if}
