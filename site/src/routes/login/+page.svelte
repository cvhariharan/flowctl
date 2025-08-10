<script lang="ts">
  import { apiClient } from '$lib/apiClient';
  import { goto, invalidateAll } from '$app/navigation';
  import { handleInlineError } from '$lib/utils/errorHandling';
  import Logo from '$lib/components/login/Logo.svelte';
  import LoginCard from '$lib/components/login/LoginCard.svelte';
  import Footer from '$lib/components/login/Footer.svelte';

  let username = $state('');
  let password = $state('');
  let loading = $state(false);

  const submit = async (event: SubmitEvent) => {
    event.preventDefault();
    if (!username || !password) {
      return;
    }

    loading = true;

    try {
      await apiClient.auth.login({ username, password });
      await invalidateAll();
      goto('/view/default/flows');
    } catch (err) {
      handleInlineError(err, 'Unable to Sign In');
    } finally {
      loading = false;
    }
  };

</script>

<svelte:head>
  <title>Login - Flowctl</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@tabler/icons-webfont@latest/tabler-icons.min.css">
</svelte:head>

<div class="min-h-screen flex items-center justify-center bg-slate-50">
  <div class="w-full max-w-md">
    <Logo />
    <LoginCard 
      onSubmit={submit} 
      {loading} 
      bind:username 
      bind:password 
    />
    <Footer />
  </div>
</div>