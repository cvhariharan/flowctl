<script lang="ts">
  import UserDropdown from './UserDropdown.svelte';
  
  let { 
    breadcrumbs = [], 
    actions = [],
    showUserDropdown = true,
    children
  }: { 
    breadcrumbs?: string[],
    actions?: Array<{ label: string, onClick: () => void, variant?: 'primary' | 'secondary' }>,
    showUserDropdown?: boolean,
    children?: any
  } = $props();
</script>

<!-- Header -->
<header class="bg-white shadow-sm border-b border-gray-200 px-6 py-4">
  <div class="flex items-center justify-between">
    <div class="flex items-center gap-5">
      {#if breadcrumbs.length > 0}
        <div class="flex items-center text-sm text-gray-500">
          {#each breadcrumbs as crumb, index}
            <span class={index === breadcrumbs.length - 1 ? 'text-gray-900' : ''}>{crumb}</span>
            {#if index < breadcrumbs.length - 1}
              <span class="mx-2">/</span>
            {/if}
          {/each}
        </div>
      {/if}
    </div>
    
    <div class="flex items-center space-x-4">
      {#if children}
        {@render children()}
      {/if}
      
      {#each actions as action}
        <button 
          onclick={action.onClick}
          class="inline-flex items-center gap-2 px-4 py-2 rounded-md transition-colors {action.variant === 'primary' ? 'bg-blue-600 text-white hover:bg-blue-700' : 'bg-white border border-gray-300 text-gray-700 hover:bg-gray-50'}"
        >
          {action.label}
        </button>
      {/each}
      
      {#if showUserDropdown}
        <UserDropdown />
      {/if}
    </div>
  </div>
</header>