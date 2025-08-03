<script lang="ts">
  import type { ApprovalResp } from '$lib/types';
  
  let { row }: { row: ApprovalResp } = $props();
</script>

<div class="flex items-center">
  <div class="w-10 h-10 bg-purple-100 rounded-lg flex items-center justify-center mr-3">
    <svg class="w-5 h-5 text-purple-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"/>
    </svg>
  </div>
  <div>
    <div class="text-sm font-medium text-gray-900">{row.action_id}</div>
    <div class="text-sm text-gray-500">{row.id}</div>
  </div>
</div>