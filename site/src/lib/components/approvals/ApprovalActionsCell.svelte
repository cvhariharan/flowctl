<script lang="ts">
  import type { ApprovalResp } from '$lib/types';
  import { getContext } from 'svelte';
  
  let { 
    row,
    value // Required by Table component interface but not used
  }: { 
    row: ApprovalResp;
    value?: any;
  } = $props();

  // Get the action functions from context
  const { handleApprove, handleReject } = getContext<{
    handleApprove: (id: string) => void;
    handleReject: (id: string) => void;
  }>('approvalActions');
</script>

<div class="flex space-x-2">
  {#if row.status === 'pending'}
    <button
      onclick={() => handleApprove(row.id)}
      class="text-success-600 hover:text-success-800"
    >
      Approve
    </button>
    <button
      onclick={() => handleReject(row.id)}
      class="text-danger-600 hover:text-danger-800"
    >
      Reject
    </button>
  {:else}
    <span class="text-gray-400">No actions</span>
  {/if}
</div>