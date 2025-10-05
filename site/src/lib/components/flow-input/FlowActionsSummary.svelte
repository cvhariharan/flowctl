<script lang="ts">
  import type { FlowAction } from '$lib/types';
  
  let { 
    actions,
    title = 'Flow Actions'
  }: {
    actions: FlowAction[],
    title?: string
  } = $props();
</script>

{#if actions && actions.length > 0}
  <div class="bg-white rounded-lg border border-gray-200 mt-6">
    <div class="px-4 py-3 border-b border-gray-200">
      <h3 class="text-sm font-semibold text-gray-900">
        {title} ({actions.length})
      </h3>
    </div>
    <div class="divide-y divide-gray-200">
      {#each actions as action, index}
        <div class="px-4 py-3 flex items-center justify-between hover:bg-gray-50 transition-colors">
          <div class="flex items-center gap-3">
            <div class="w-6 h-6 bg-primary-100 text-primary-600 rounded flex items-center justify-center text-xs font-medium">
              {index + 1}
            </div>
            <div class="text-sm text-gray-900">{action.name}</div>
          </div>
          <span class="inline-flex px-2 py-1 text-xs font-medium rounded bg-gray-100 text-gray-700">
            {action.executor}
          </span>
        </div>
      {/each}
    </div>
  </div>
{/if}