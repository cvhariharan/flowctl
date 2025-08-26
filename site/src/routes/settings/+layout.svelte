<script lang="ts">
    import Sidebar from '$lib/components/shared/Sidebar.svelte';
    import { getContext } from 'svelte';
    
    let { children, data } = $props();
    let namespace = getContext('namespace') as string || 'default';
  </script>
  
  <svelte:head>
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@tabler/icons-webfont@latest/tabler-icons.min.css">
  </svelte:head>
  
  <div class="flex h-screen bg-gray-50">
    <Sidebar namespace={namespace} />
    
    <div class="flex-1 flex flex-col overflow-hidden">
      <main class="flex-1 overflow-y-auto">
        {@render children()}
      </main>
    </div>
  </div>