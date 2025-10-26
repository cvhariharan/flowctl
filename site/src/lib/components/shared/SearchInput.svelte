<script lang="ts">
  let { 
    value = $bindable(''),
    placeholder = 'Search...',
    loading = false,
    onSearch,
    debounceMs = 300
  }: {
    value: string,
    placeholder?: string,
    loading?: boolean,
    onSearch: (query: string) => void,
    debounceMs?: number
  } = $props();

  let debounceTimer: number;

  const handleInput = (event: Event) => {
    const target = event.target as HTMLInputElement;
    value = target.value;
    
    clearTimeout(debounceTimer);
    debounceTimer = setTimeout(() => {
      onSearch(value.trim());
    }, debounceMs);
  };
</script>

<div class="max-w-md">
  <div class="relative">
    <input
      type="text"
      {placeholder}
      {value}
      oninput={handleInput}
      class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent"
    />
    {#if loading}
      <div class="absolute right-3 top-1/2 transform -translate-y-1/2">
        <svg class="animate-spin h-4 w-4 text-gray-400" fill="none" viewBox="0 0 24 24">
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
          <path class="opacity-75" fill="currentColor" d="m4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
        </svg>
      </div>
    {/if}
  </div>
</div>