<script lang="ts">
  import { apiClient } from '$lib/apiClient';
  import { goto, invalidateAll } from '$app/navigation';
  import { page } from '$app/stores';
  import { handleInlineError } from '$lib/utils/errorHandling';
  import { getDefaultNamespace } from '$lib/utils/navigation';
  import { isLoading } from '$lib/stores/auth';
  import { appInfo } from '$lib/stores/auth';
  import { onMount } from 'svelte';
  import Logo from '$lib/components/shared/Logo.svelte';
  import LoginCard from '$lib/components/login/LoginCard.svelte';
  import ThemeToggle from '$lib/components/shared/ThemeToggle.svelte';

  let username = $state('');
  let password = $state('');
  let loading = $state(false);
  let error = $state('');

  const redirectUrl = $derived($page.url.searchParams.get('redirect_url'));

  // Reset loading state when landing on login page
  onMount(() => {
    isLoading.set(false);
  });

  const submit = async (event: SubmitEvent) => {
    event.preventDefault();
    if (!username || !password) {
      return;
    }

    loading = true;

    try {
      await apiClient.auth.login({ username, password });
      await invalidateAll();
      // Keep loading state active until navigation completes
      if (redirectUrl && redirectUrl.startsWith('/')) {
        goto(redirectUrl);
      } else {
        const namespace = await getDefaultNamespace();
        goto(`/view/${encodeURIComponent(namespace)}/flows`);
      }
    } catch (err) {
      handleInlineError(err, 'Unable to Sign In');
      loading = false;
    }
  };

</script>

<svelte:head>
  <title>Login - {$appInfo?.branding?.app_name || "Flowctl"}</title>
</svelte:head>

<main class="login-page">
  <section class="login-container">
    <div class="mb-8 flex justify-center p-4">
      <Logo height="3.5rem" />
    </div>
    <LoginCard
      onSubmit={submit}
      {loading}
      {error}
      bind:username
      bind:password
      {redirectUrl}
    />
    <footer class="mt-6 align-center">
      <p class="text-lighter text-sm">Need help? Contact your system administrator</p>
    </footer>
  </section>
  <div class="theme-toggle-pos">
    <ThemeToggle collapsed={true} />
  </div>
</main>

<style>
  .login-page {
    min-height: 100vh;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--muted);
    padding-inline: var(--space-4);
    position: relative;
  }

  .login-container {
    width: 100%;
    max-width: 28rem;
  }

  .theme-toggle-pos {
    position: absolute;
    bottom: var(--space-4);
    right: var(--space-4);
  }
</style>
