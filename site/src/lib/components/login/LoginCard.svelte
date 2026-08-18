<script lang="ts">
  import IconKey from '@tabler/icons-svelte/icons/key';
  import { onMount } from 'svelte';
  import { apiClient } from '$lib/apiClient';
  import type { SSOProvider } from '$lib/types';

  let {
    onSubmit,
    error,
    loading,
    username = $bindable(''),
    password = $bindable(''),
    redirectUrl = null
  }: {
    onSubmit: (event: SubmitEvent) => void,
    error: string,
    loading: boolean,
    username: string,
    password: string,
    redirectUrl: string | null
  } = $props();

  let ssoProviders: SSOProvider[] = $state([]);
  let oidcLoadingProvider = $state<string | null>(null);

  onMount(async () => {
    try {
      ssoProviders = await apiClient.auth.getSSOProviders();
    } catch (err) {
      console.error('Failed to fetch SSO providers:', err);
    }
  });

  const handleOIDCLogin = (providerId: string) => {
    oidcLoadingProvider = providerId;
    const url = redirectUrl
      ? `/login/oidc/${providerId}?redirect_url=${encodeURIComponent(redirectUrl)}`
      : `/login/oidc/${providerId}`;
    window.location.href = url;
  };
</script>

<!-- Login Card -->
<article class="card login-card">
  <form onsubmit={onSubmit} class="vstack gap-5" aria-label="Login form">
    <!-- Error Message -->
    {#if error}
      <div role="alert" data-variant="error" aria-live="assertive">
        <div>{error}</div>
      </div>
    {/if}

    <!-- Username Field -->
    <div>
      <label for="username">
        Username
      </label>
      <input
        type="text"
        bind:value={username}
        id="username"
        name="username"
        required
        placeholder="Enter your username"
      />
    </div>

    <!-- Password Field -->
    <div>
      <label for="password">
        Password
      </label>
      <input
        type="password"
        bind:value={password}
        id="password"
        name="password"
        required
        placeholder="Enter your password"
      />
    </div>

    <!-- Sign In Button -->
    <button
      type="submit"
      disabled={loading}
      class="login-submit"
    >
      {#if loading}
        <span class="hstack gap-2 justify-center" aria-busy="true" data-spinner="small">Signing In...</span>
      {:else}
        Sign In
      {/if}
    </button>

    {#each ssoProviders as provider}
      <button
        type="button"
        data-variant="secondary"
        onclick={() => handleOIDCLogin(provider.id)}
        disabled={oidcLoadingProvider !== null}
        aria-label={provider.label}
      >
        {#if oidcLoadingProvider === provider.id}
          <span class="hstack gap-2 justify-center" aria-busy="true" data-spinner="small">Redirecting...</span>
        {:else}
          <span class="hstack gap-2 justify-center">
            <IconKey size={16} aria-hidden="true" />
            {provider.label}
          </span>
        {/if}
      </button>
    {/each}
  </form>
</article>

<style>
  .login-card {
    padding: var(--space-8);
  }

  .login-submit {
    text-align: center;
  }
</style>
