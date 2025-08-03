<script lang="ts">
  let { 
    icon,
    message,
    description
  }: {
    icon?: string,
    message: string,
    description?: string
  } = $props();

  const defaultIcon = `
    <svg class="mx-auto h-12 w-12 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"/>
    </svg>
  `;
</script>

<div class="text-center py-12 text-gray-500">
  {@html icon || defaultIcon}
  <p class="mt-2 text-base">{message}</p>
  {#if description}
    <p class="mt-1 text-sm text-gray-400">{description}</p>
  {/if}
</div>