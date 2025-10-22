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
      schedules: string[];
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

  function addSchedule() {
    if (!metadata.schedules) {
      metadata.schedules = [];
    }
    metadata.schedules.push('');
  }

  function removeSchedule(index: number) {
    metadata.schedules.splice(index, 1);
  }

  function updateSchedule(index: number, value: string) {
    if (!metadata.schedules) {
      metadata.schedules = [];
    }
    metadata.schedules[index] = value;
  }

  // Reactive validation for schedules using Svelte 5 syntax
  let scheduleValidations = $derived(
    metadata.schedules?.map(schedule => ({
      schedule,
      isValid: schedule === '' || isValidCronExpression(schedule)
    })) || []
  );
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
      <div class="flex items-center justify-between mb-2">
        <label class="block text-sm font-medium text-gray-700">
          Cron Schedules
          <span class="text-sm text-gray-500 font-normal">(optional)</span>
        </label>
        <button
          type="button"
          onclick={addSchedule}
          class="text-xs text-primary-600 hover:text-primary-700 font-medium cursor-pointer"
        >
          + Add Schedule
        </button>
      </div>

      <div class="space-y-3">
        {#each metadata.schedules || [] as schedule, index}
          {@const validation = scheduleValidations[index]}
          <div class="flex items-start gap-2">
            <div class="flex-1">
              <input
                type="text"
                value={schedule}
                oninput={(e) => updateSchedule(index, e.currentTarget.value)}
                class="w-full px-3 py-2 border rounded-md focus:outline-none focus:ring-2 {validation?.isValid ? 'border-gray-300 focus:ring-primary-500 focus:border-transparent' : 'border-danger-300 focus:ring-danger-500 focus:border-transparent'}"
                placeholder="0 2 * * * (daily at 2:00 AM)"
              />
              {#if schedule && !validation?.isValid}
                <p class="text-xs text-danger-600 mt-1">
                  Invalid cron expression. Use format: minute hour day month weekday (e.g., "0 2 * * *")
                </p>
              {/if}
            </div>
            <button
              type="button"
              onclick={() => removeSchedule(index)}
              class="mt-2 text-gray-400 hover:text-danger-600 cursor-pointer"
            >
              <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>
        {/each}

        {#if !metadata.schedules || metadata.schedules.length === 0}
          <div class="text-center py-6 border-2 border-dashed border-gray-300 rounded-md">
            <svg class="mx-auto h-8 w-8 text-gray-400 mb-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            <p class="text-sm text-gray-500 mb-2">No schedules defined</p>
            <button
              type="button"
              onclick={addSchedule}
              class="text-sm text-primary-600 hover:text-primary-700 font-medium cursor-pointer"
            >
              Add your first schedule
            </button>
          </div>
        {/if}
      </div>

      <p class="text-xs text-gray-500 mt-2">
        Use cron expression format. You can add multiple schedules for different execution times.
        <br>
        Examples: <code class="bg-gray-100 px-1 rounded">0 2 * * *</code> (daily 2AM),
        <code class="bg-gray-100 px-1 rounded">0 */6 * * *</code> (every 6 hours)
      </p>
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
