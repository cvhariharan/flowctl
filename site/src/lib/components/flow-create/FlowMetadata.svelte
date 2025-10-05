<script lang="ts">
  import { createSlug, isValidCronExpression } from '$lib/utils';

  let {
    metadata = $bindable(),
    inputs = [],
    readonly = false
  }: {
    metadata: {
      id: string;
      name: string;
      description: string;
      schedule: string;
      namespace: string;
    };
    inputs?: any[];
    readonly?: boolean;
  } = $props();

  // Compute schedulable status based on inputs
  let isSchedulable = $derived(
    inputs.length === 0 || inputs.every(input => input.default && input.default.trim() !== '')
  );


  function updateName(value: string) {
    if (readonly) return;
    metadata.name = value;
    // Auto-generate ID from name
    metadata.id = createSlug(value);
  }

  function updateDescription(value: string) {
    if (readonly) return;
    metadata.description = value;  
  }

  function updateSchedule(value: string) {
    metadata.schedule = value;
  }

  // Reactive validation for schedule using Svelte 5 syntax
  let isValidSchedule = $derived(isValidCronExpression(metadata.schedule));
</script>

<!-- Flow Metadata Section -->
<div>
  <div class="grid grid-cols-1 gap-6">
    <div>
      <label for="flow-name" class="block text-sm font-medium text-gray-700 mb-2">Flow Name *</label>
      <input 
        type="text" 
        id="flow-name" 
        value={metadata.name}
        oninput={(e) => updateName(e.currentTarget.value)}
        class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent {readonly ? 'bg-gray-50 cursor-not-allowed' : ''}"
        placeholder="My Flow Name"
        disabled={readonly}
      />
    </div>
    <div>
      <label for="flow-description" class="block text-sm font-medium text-gray-700 mb-2">Description</label>
      <textarea 
        id="flow-description" 
        value={metadata.description}
        oninput={(e) => updateDescription(e.currentTarget.value)}
        class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent resize-none h-20 {readonly ? 'bg-gray-50 cursor-not-allowed' : ''}"
        placeholder="Describe what this flow does..."
        disabled={readonly}
      ></textarea>
    </div>
    {#if isSchedulable}
    <div>
      <label for="flow-schedule" class="block text-sm font-medium text-gray-700 mb-2">
        Cron Schedule
        <span class="text-sm text-gray-500 font-normal">(optional)</span>
      </label>
      <input 
        type="text" 
        id="flow-schedule" 
        value={metadata.schedule}
        oninput={(e) => updateSchedule(e.currentTarget.value)}
        class="w-full px-3 py-2 border rounded-md focus:outline-none focus:ring-2 {isValidSchedule ? 'border-gray-300 focus:ring-primary-500 focus:border-transparent' : 'border-danger-300 focus:ring-danger-500 focus:border-transparent'}"
        placeholder="0 2 * * * (daily at 2:00 AM)"
      />
      {#if metadata.schedule && !isValidSchedule}
        <p class="text-xs text-danger-600 mt-1">
          Invalid cron expression. Use format: minute hour day month weekday (e.g., "0 2 * * *")
        </p>
      {:else}
        <p class="text-xs text-gray-500 mt-1">
          Use cron expression format. Leave empty for manual execution only.
          <br>
          Examples: <code class="bg-gray-100 px-1 rounded">0 2 * * *</code> (daily 2AM), 
          <code class="bg-gray-100 px-1 rounded">0 */6 * * *</code> (every 6 hours)
        </p>
      {/if}
    </div>
    {:else}
    <div>
      <div class="bg-warning-50 border border-warning-200 rounded-md p-4">
        <div class="flex items-start">
          <div class="flex-shrink-0">
            <svg class="h-5 w-5 text-warning-400" viewBox="0 0 20 20" fill="currentColor">
              <path fill-rule="evenodd" d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z" clip-rule="evenodd" />
            </svg>
          </div>
          <div class="ml-3">
            <h3 class="text-sm font-medium text-warning-800">
              Flow Not Schedulable
            </h3>
            <div class="mt-2 text-sm text-warning-700">
              <p>This flow cannot be scheduled because it has inputs without default values. To make this flow schedulable, ensure all inputs have default values.</p>
            </div>
          </div>
        </div>
      </div>
    </div>
    {/if}
  </div>
</div>