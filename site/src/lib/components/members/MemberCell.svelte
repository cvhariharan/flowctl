<script lang="ts">
  import type { NamespaceMemberResp } from '$lib/types';
  
  let { row }: { row: NamespaceMemberResp } = $props();
</script>

<div class="flex items-center">
  <div class="w-10 h-10 rounded-lg flex items-center justify-center mr-3 {row.subject_type === 'user' ? 'bg-primary-100' : 'bg-primary-100'}">
    {#if row.subject_type === 'user'}
      <svg class="w-5 h-5 text-primary-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"/>
      </svg>
    {:else}
      <svg class="w-5 h-5 text-primary-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z"/>
      </svg>
    {/if}
  </div>
  <div>
    <div class="text-sm font-medium text-gray-900">{row.subject_name}</div>
    <div class="text-sm text-gray-500">{row.subject_id}</div>
  </div>
</div>