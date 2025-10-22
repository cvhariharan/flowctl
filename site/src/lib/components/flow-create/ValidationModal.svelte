<script lang="ts">
  let {
    show = $bindable(),
    validationResult
  }: {
    show: boolean;
    validationResult: {
      success: boolean;
      errors: string[];
    };
  } = $props();

  function close() {
    show = false;
  }
</script>

{#if show}
  <!-- Validation Result Modal -->
  <div class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
    <div class="bg-white rounded-xl p-6 max-w-2xl w-full mx-4">
      <div class="flex items-center mb-4">
        <div 
          class="w-12 h-12 rounded-full flex items-center justify-center mr-4 {validationResult.success ? 'bg-success-100' : 'bg-danger-100'}"
        >
          {#if validationResult.success}
            <svg class="w-6 h-6 text-success-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
          {:else}
            <svg class="w-6 h-6 text-danger-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
          {/if}
        </div>
        <div>
          <h3 class="text-lg font-semibold text-gray-900">
            {validationResult.success ? 'Validation Passed' : 'Validation Failed'}
          </h3>
          <p class="text-sm text-gray-600">
            {validationResult.success ? 'Your flow definition is valid.' : 'Please fix the following issues:'}
          </p>
        </div>
      </div>

      {#if !validationResult.success && validationResult.errors.length > 0}
        <div class="space-y-2 mb-4">
          {#each validationResult.errors as error}
            <div class="flex items-start gap-2 text-sm">
              <svg class="w-4 h-4 text-danger-500 mt-0.5 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                  d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
              <span class="text-gray-700">{error}</span>
            </div>
          {/each}
        </div>
      {/if}

      <div class="flex justify-end">
        <button
          onclick={close}
          class="px-4 py-2 bg-gray-600 text-white rounded-md hover:bg-gray-700 transition-colors cursor-pointer"
        >
          Close
        </button>
      </div>
    </div>
  </div>
{/if}