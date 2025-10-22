<script lang="ts">
  let { 
    title = 'Error',
    message,
    onDismiss,
    dismissible = false
  }: {
    title?: string,
    message: string,
    onDismiss?: () => void,
    dismissible?: boolean
  } = $props();
</script>

<div class="mb-6 bg-danger-50 border border-danger-100 rounded-lg p-4" role="alert" aria-live="assertive">
  <div class="flex">
    <svg class="w-5 h-5 text-danger-500 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path>
    </svg>
    <div class="ml-3 flex-1">
      <h3 class="text-sm font-medium text-danger-900">{title}</h3>
      <p class="mt-1 text-sm text-danger-800">{message}</p>
    </div>
    {#if dismissible && onDismiss}
      <button
        onclick={onDismiss}
        class="ml-auto -mx-1.5 -my-1.5 bg-danger-50 text-danger-500 rounded-lg focus:ring-2 focus:ring-danger-600 p-1.5 hover:bg-danger-100 inline-flex h-8 w-8 cursor-pointer"
      >
        <span class="sr-only">Dismiss</span>
        <svg class="w-3 h-3" aria-hidden="true" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 14 14">
          <path stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="m1 1 6 6m0 0 6 6M7 7l6-6M7 7l-6 6"/>
        </svg>
      </button>
    {/if}
  </div>
</div>