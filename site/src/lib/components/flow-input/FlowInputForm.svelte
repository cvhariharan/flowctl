<script lang="ts">
  import type { FlowInput } from '$lib/types';
  import { ApiError } from '$lib/apiClient';
  import { handleInlineError } from '$lib/utils/errorHandling';

  let { inputs, namespace, flowId }: { inputs: FlowInput[], namespace: string, flowId: string } = $props();

  let loading = $state(false);
  let errors = $state<Record<string, string>>({});

  const submit = async (event: SubmitEvent) => {
    event.preventDefault();
    loading = true;
    errors = {};
    
    const form = event.target as HTMLFormElement;
    const formData = new FormData(form);
    
    try {
      const response = await fetch(`/api/v1/${namespace}/trigger/${flowId}`, {
        method: 'POST',
        body: formData,
      });

      if (!response.ok) {
        const errorData = await response.json().catch(() => ({}));
        
        // Handle validation errors with field details - show inline
        if (errorData.details && errorData.details.field && errorData.details.error) {
          errors[errorData.details.field] = errorData.details.error;
        } 
        // For non-validation errors, use common error handling
        else {
          const apiError = new ApiError(response.status, response.statusText, errorData);
          handleInlineError(apiError, 'Unable to Start Flow');
        }
      } else {
        const data = await response.json();
        // Success - redirect to results page
        window.location.href = `/view/${namespace}/results/${flowId}/${data.exec_id}`;
      }
    } catch (error) {
      handleInlineError(error, 'Unable to Start Flow');
    } finally {
      loading = false;
    }
  };

</script>

<div class="bg-white rounded-lg shadow-sm border border-gray-200 overflow-hidden">
  <div class="px-6 py-4 border-b border-gray-200 bg-gray-50">
    <h2 class="text-lg font-semibold text-gray-900">Configuration Parameters</h2>
    <p class="text-sm text-gray-600 mt-1">Configure the inputs for this flow execution</p>
  </div>

  <form onsubmit={submit} class="p-6 space-y-6">
    {#if errors.general}
      <div class="p-3 rounded-md bg-red-50 border border-red-200">
        <div class="text-sm text-red-700">{errors.general}</div>
      </div>
    {/if}

    {#each inputs as input (input.name)}
      <div>
        <label for={input.name} class="block text-sm font-medium text-gray-700 mb-2">
          {input.label || input.name}
          {#if input.required}
            <span class="text-red-500">*</span>
          {/if}
        </label>

        {#if errors[input.name]}
          <p class="text-sm text-red-600 mb-2">{errors[input.name]}</p>
        {/if}

        {#if input.type === 'string' || input.type === 'number'}
          <input
            type={input.type === 'string' ? 'text' : 'number'}
            id={input.name}
            name={input.name}
            placeholder={input.description || ''}
            required={input.required}
            class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
          />
        {:else if input.type === 'checkbox'}
          <div class="flex items-center">
            <input
              type="checkbox"
              id={input.name}
              name={input.name}
              value="true"
              class="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded"
            />
          </div>
        {:else if input.type === 'select' && input.options}
          <select
            id={input.name}
            name={input.name}
            required={input.required}
            class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
          >
            <option value="">Select an option</option>
            {#each input.options as option}
              <option value={option}>{option}</option>
            {/each}
          </select>
          {:else if input.type === 'file'}
          <div class="flex items-center">
            <input
              type="file"
              id={input.name}
              name={input.name}
              required={input.required}
              class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent file:mr-4 file:py-2 file:px-4 file:rounded-full file:border-0 file:text-sm file:font-semibold file:bg-blue-50 file:text-blue-700 hover:file:bg-blue-100"
            />
          </div>
        {:else}
          <!-- Fallback for other input types -->
          <input
            type="text"
            id={input.name}
            name={input.name}
            placeholder={input.description || ''}
            required={input.required}
            class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
          />
        {/if}

        {#if input.description}
          <p class="text-sm text-gray-500 mt-1">{input.description}</p>
        {/if}
      </div>
    {/each}

    <div class="flex gap-3 pt-6 border-t border-gray-200">
      <button
        type="button"
        onclick={() => window.history.back()}
        class="flex-1 px-4 py-2 bg-white border border-gray-300 text-gray-700 rounded-md hover:bg-gray-50 transition-colors"
      >
        Cancel
      </button>
      <button
        type="submit"
        disabled={loading}
        class="flex-1 inline-flex items-center justify-center gap-2 px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 transition-colors disabled:opacity-50"
      >
        {#if loading}
          <svg class="animate-spin h-5 w-5 text-white" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
            <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
          </svg>
          Starting Flow...
        {:else}
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z"/>
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z"/>
          </svg>
          Run Flow
        {/if}
      </button>
    </div>
  </form>
</div>