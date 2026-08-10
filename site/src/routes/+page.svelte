<script lang="ts">
  import { goto } from '$app/navigation';
  import { isAuthenticated, isLoading } from '$lib/stores/auth';
  import { getDefaultNamespace } from '$lib/utils/navigation';
  import Logo from '$lib/components/shared/Logo.svelte';
  import Footer from '$lib/components/shared/Footer.svelte';

  // Wait for auth loading to complete
  $effect(() => {
    if ($isLoading) {
      return;
    }

    if (!$isAuthenticated) {
      goto('/login');
      return;
    }

    getDefaultNamespace()
      .then((namespace) => goto(`/view/${encodeURIComponent(namespace)}/flows`))
      .catch(() => goto('/login'));
  });
</script>

<svelte:head>
  <title>Flowctl</title>
</svelte:head>

<div class="loading-page">
  <div class="loading-body vstack items-center gap-6">
    <Logo height="4rem" />
    <div class="hstack justify-center gap-4" aria-busy="true">Loading...</div>
  </div>
  <Footer />
</div>

<style>
  .loading-page {
    min-height: 100vh;
    display: flex;
    flex-direction: column;
    background: var(--card);
  }

  .loading-body {
    flex: 1;
    justify-content: center;
  }
</style>
