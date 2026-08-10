<script lang="ts">
  import Sidebar from '$lib/components/shared/Sidebar.svelte';
  import Footer from '$lib/components/shared/Footer.svelte';
  import { selectedNamespace } from '$lib/stores/namespace';

  let { children, data } = $props();
</script>

<svelte:head>
  <meta name="viewport" content="width=1024, initial-scale=1.0, user-scalable=yes" />
</svelte:head>

<div data-sidebar-layout>
  <Sidebar namespace={$selectedNamespace} />
  <main class="app-content">
    {@render children()}
    <Footer />
  </main>
</div>
