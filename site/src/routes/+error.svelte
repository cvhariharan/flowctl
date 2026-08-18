<script lang="ts">
  import { page } from '$app/state';
  import { goto } from '$app/navigation';
  import { isAuthenticated } from '$lib/stores/auth';
  import { appInfo } from '$lib/stores/auth';
  import { getDefaultNamespace } from '$lib/utils/navigation';
  import IconLock from '@tabler/icons-svelte/icons/lock';
  import IconShieldX from '@tabler/icons-svelte/icons/shield-x';
  import IconFileX from '@tabler/icons-svelte/icons/file-x';
  import IconAlertTriangle from '@tabler/icons-svelte/icons/alert-triangle';
  import IconLogin from '@tabler/icons-svelte/icons/login';
  import IconHome from '@tabler/icons-svelte/icons/home';

  const error = page.error;
  const status = page.status;
  const errorCode = error?.code;

  const getErrorDetails = () => {
    if (status === 401) {
      return {
        IconComponent: IconLock,
        color: 'var(--warning)',
        bg: 'color-mix(in srgb, var(--warning) 15%, transparent)',
        title: 'Authentication Required',
        message: error?.message || 'Please log in to access this resource',
        showLoginButton: true
      };
    }

    if (status === 403) {
      return {
        IconComponent: IconShieldX,
        color: 'var(--danger)',
        bg: 'color-mix(in srgb, var(--danger) 15%, transparent)',
        title: 'Access Denied',
        message: error?.message || 'You do not have permission to access this resource',
        showLoginButton: false
      };
    }

    if (status === 404) {
      return {
        IconComponent: IconFileX,
        color: 'var(--muted-foreground)',
        bg: 'var(--faint)',
        title: 'Page Not Found',
        message: error?.message || 'The page you are looking for does not exist',
        showLoginButton: false
      };
    }

    return {
      IconComponent: IconAlertTriangle,
      color: 'var(--danger)',
      bg: 'color-mix(in srgb, var(--danger) 15%, transparent)',
      title: 'Something went wrong',
      message: error?.message || 'An unexpected error occurred',
      showLoginButton: false
    };
  };

  const errorDetails = getErrorDetails();

  const handleGoHome = async () => {
    if ($isAuthenticated) {
      const namespace = await getDefaultNamespace();
      goto(`/view/${encodeURIComponent(namespace)}/flows`);
    } else {
      goto('/');
    }
  };

  const handleLogin = () => {
    goto('/login');
  };
</script>

<svelte:head>
  <title>Error - {$appInfo?.branding?.app_name || "Flowctl"}</title>
</svelte:head>

<div class="error-page vstack items-center justify-center">
  <div style="max-width: 32rem; width: 100%; text-align: center">
    <!-- Error Icon -->
    <div class="mb-8">
      <div style="margin-inline: auto; width: 6rem; height: 6rem; border-radius: 50%; display: inline-flex; align-items: center; justify-content: center; background: {errorDetails.bg}">
        <errorDetails.IconComponent style="color: {errorDetails.color}" size={48} />
      </div>
    </div>

    <!-- Error Content -->
    <div class="mb-8">
      <h1 class="mb-4">
        {errorDetails.title}
      </h1>
      <p class="text-light mb-2" style="font-size: var(--text-5);">
        {errorDetails.message}
      </p>

      <!-- Show error code if available -->
      {#if errorCode}
        <p class="text-light mt-2 font-mono text-sm">
          Error Code: {errorCode}
        </p>
      {/if}
    </div>

    <!-- Action Buttons -->
    <div class="hstack gap-3 justify-center flex-wrap">
      {#if errorDetails.showLoginButton}
        <button onclick={handleLogin}>
          <span class="hstack gap-2">
            <IconLogin size={18} />
            Log In
          </span>
        </button>
      {/if}

      <button
        onclick={handleGoHome}
        data-variant="secondary"
      >
        <span class="hstack gap-2">
          <IconHome size={18} />
          {$isAuthenticated ? 'Dashboard' : 'Home'}
        </span>
      </button>
    </div>
  </div>
</div>

<style>
  .error-page {
    min-height: 100vh;
    background: var(--muted);
    padding-inline: var(--space-4);
  }
</style>
