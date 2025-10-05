<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { apiClient } from '$lib/apiClient';
  import { currentUser, isAuthenticated, isLoading } from '$lib/stores/auth';
  
  onMount(async () => {
    // Wait for auth to load
    if ($isLoading) {
      // Wait a bit for the auth store to be populated
      await new Promise(resolve => setTimeout(resolve, 100));
    }
    
    if (!$isAuthenticated) {
      goto('/login');
      return;
    }
    
    try {
      goto(`/view/default/flows`);
    } catch (error) {
      goto('/login');
    }
  });
</script>

<svelte:head>
  <title>Flowctl</title>
</svelte:head>

<div class="min-h-screen flex items-center justify-center bg-gray-50">
  <div class="text-center">
    <div class="w-16 h-16 mx-auto mb-4 flex items-center justify-center rounded-lg bg-primary-500">
      <span class="text-2xl font-bold text-white">F</span>
    </div>
    <h1 class="text-2xl font-bold text-gray-900 mb-2">Flowctl</h1>
    <p class="text-gray-600">Loading...</p>
  </div>
</div>