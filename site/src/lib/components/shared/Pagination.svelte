<script lang="ts">
  import { createEventDispatcher } from 'svelte';

  type Props = {
    currentPage: number;
    totalPages: number;
    loading?: boolean;
    disabled?: boolean;
  };

  let { 
    currentPage, 
    totalPages, 
    loading = false,
    disabled = false
  }: Props = $props();

  const dispatch = createEventDispatcher<{
    'page-change': { page: number };
  }>();

  const handlePageChange = (page: number) => {
    if (page !== currentPage && page >= 1 && page <= totalPages && !disabled && !loading) {
      dispatch('page-change', { page });
    }
  };

  const isPreviousDisabled = $derived(currentPage === 1 || disabled || loading);
  const isNextDisabled = $derived(currentPage === totalPages || disabled || loading);

  const getVisiblePages = () => {
    const pages: number[] = [];
    const maxVisible = 7;
    
    if (totalPages <= maxVisible) {
      for (let i = 1; i <= totalPages; i++) {
        pages.push(i);
      }
    } else {
      const start = Math.max(1, currentPage - 3);
      const end = Math.min(totalPages, start + maxVisible - 1);
      
      for (let i = start; i <= end; i++) {
        pages.push(i);
      }
    }
    
    return pages;
  };

  let visiblePages = $derived(getVisiblePages());
</script>

{#if totalPages > 1}
  <div class="flex justify-center mt-8">
    <nav class="flex items-center space-x-2">
      <button 
        onclick={() => handlePageChange(currentPage - 1)}
        disabled={isPreviousDisabled}
        class="px-3 py-2 text-sm font-medium text-gray-500 bg-white border border-gray-300 rounded-lg hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
      >
        Previous
      </button>

      {#each visiblePages as page}
        <button 
          onclick={() => handlePageChange(page)}
          disabled={disabled || loading}
          class="px-3 py-2 text-sm font-medium border border-gray-300 rounded-lg hover:bg-gray-50 disabled:cursor-not-allowed
                 {page === currentPage ? 'bg-primary-500 text-white' : 'bg-white text-gray-700'}"
        >
          {page}
        </button>
      {/each}

      <button 
        onclick={() => handlePageChange(currentPage + 1)}
        disabled={isNextDisabled}
        class="px-3 py-2 text-sm font-medium text-gray-500 bg-white border border-gray-300 rounded-lg hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
      >
        Next
      </button>
    </nav>
  </div>
{/if}