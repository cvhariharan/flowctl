<script lang="ts">
  let {
    name,
    description,
    icon = `<svg class="w-6 h-6 text-gray-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2V6zM14 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2V6zM4 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2v-2zM14 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2v-2z"/>
    </svg>`
  }: {
    name: string,
    description?: string,
    icon?: string
  } = $props();
</script>

<!-- Hero Section -->
<div class="bg-white border-b border-gray-200 px-6 py-6">
  <div class="max-w-4xl mx-auto">
    <div class="flex items-start gap-4">
      <div class="w-12 h-12 bg-primary-100 rounded-lg flex items-center justify-center flex-shrink-0">
        {@html icon}
      </div>
      <div class="flex-1 min-w-0">
        <h1 class="text-2xl font-semibold text-gray-900 mb-1">{name}</h1>
        {#if description}
          <p class="text-sm text-gray-600">{description}</p>
        {/if}
      </div>
    </div>
  </div>
</div>