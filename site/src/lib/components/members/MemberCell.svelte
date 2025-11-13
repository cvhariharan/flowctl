<script lang="ts">
  import type { NamespaceMemberResp } from '$lib/types';
  import { IconUsers, IconUser } from '@tabler/icons-svelte';

  let { row, onClick }: { row: NamespaceMemberResp; onClick?: (member: NamespaceMemberResp) => void } = $props();
</script>

<div class="flex items-center">
  <div class="w-10 h-10 rounded-lg flex items-center justify-center mr-3 bg-primary-100">
    {#if row.subject_type === 'user'}
      <IconUser class="w-5 h-5 text-primary-600" />
    {:else}
      <IconUsers class="w-5 h-5 text-primary-600" />
    {/if}
  </div>
  <div class="transition-colors">
    <div class="text-sm {onClick ? 'cursor-pointer hover:text-primary-600' : ''} font-medium text-gray-900" onclick={() => onClick?.(row)}>{row.subject_name}</div>
    <div class="text-sm text-gray-500">{row.subject_id}</div>
  </div>
</div>
