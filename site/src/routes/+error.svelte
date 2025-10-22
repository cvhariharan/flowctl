<script lang="ts">
  import { page } from '$app/state';
  import { goto } from '$app/navigation';
  import { isAuthenticated } from '$lib/stores/auth';
  import {
    IconLock,
    IconShieldX,
    IconFileX,
    IconAlertTriangle,
    IconLogin,
    IconHome
  } from '@tabler/icons-svelte';

  const error = page.error;
  const status = page.status;
  const errorCode = error?.code;
  
  // Get error details based on status and code
  const getErrorDetails = () => {
    if (status === 401) {
      return {
        IconComponent: IconLock,
        iconColor: 'text-warning-600',
        bgColor: 'bg-warning-100',
        title: 'Authentication Required',
        message: error?.message || 'Please log in to access this resource',
        showLoginButton: true
      };
    }
    
    if (status === 403) {
      return {
        IconComponent: IconShieldX,
        iconColor: 'text-danger-600',
        bgColor: 'bg-danger-100',
        title: 'Access Denied',
        message: error?.message || 'You do not have permission to access this resource',
        showLoginButton: false
      };
    }
    
    if (status === 404) {
      return {
        IconComponent: IconFileX,
        iconColor: 'text-gray-600',
        bgColor: 'bg-gray-100',
        title: 'Page Not Found',
        message: error?.message || 'The page you are looking for does not exist',
        showLoginButton: false
      };
    }
    
    // Default error
    return {
      IconComponent: IconAlertTriangle,
      iconColor: 'text-danger-600',
      bgColor: 'bg-danger-100',
      title: 'Something went wrong',
      message: error?.message || 'An unexpected error occurred',
      showLoginButton: false
    };
  };

  const errorDetails = getErrorDetails();

  const handleGoHome = () => {
    if ($isAuthenticated) {
      goto('/view/default/flows');
    } else {
      goto('/');
    }
  };

  const handleLogin = () => {
    goto('/login');
  };
</script>

<svelte:head>
  <title>Error - Flowctl</title>
</svelte:head>

<div class="min-h-screen flex items-center justify-center bg-gray-50 px-4">
  <div class="max-w-lg w-full text-center">
    <!-- Error Icon -->
    <div class="mb-8">
      <div class="mx-auto w-24 h-24 {errorDetails.bgColor} rounded-full flex items-center justify-center">
        <errorDetails.IconComponent class="text-4xl {errorDetails.iconColor}" size={48} />
      </div>
    </div>

    <!-- Error Content -->
    <div class="mb-8">
      <h1 class="text-3xl font-bold text-gray-900 mb-4">
        {errorDetails.title}
      </h1>
      <p class="text-lg text-gray-600 mb-2">
        {errorDetails.message}
      </p>
      
      <!-- Show error code if available -->
      {#if errorCode}
        <p class="text-sm text-gray-500 mt-2 font-mono">
          Error Code: {errorCode}
        </p>
      {/if}
    </div>

    <!-- Action Buttons -->
    <div class="flex flex-col sm:flex-row gap-3 justify-center">
      {#if errorDetails.showLoginButton}
        <button
          onclick={handleLogin}
          class="inline-flex items-center gap-2 px-6 py-3 bg-primary-500 text-white rounded-md hover:bg-primary-600 transition-colors font-medium cursor-pointer"
        >
          <IconLogin size={18} />
          Log In
        </button>
      {/if}
      
      <button
        onclick={handleGoHome}
        class="inline-flex items-center gap-2 px-6 py-3 bg-white border border-gray-300 text-gray-700 rounded-md hover:bg-gray-50 transition-colors font-medium cursor-pointer"
      >
        <IconHome size={18} />
        {$isAuthenticated ? 'Dashboard' : 'Home'}
      </button>
    </div>

  </div>
</div>