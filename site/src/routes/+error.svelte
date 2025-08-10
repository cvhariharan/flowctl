<script lang="ts">
  import { page } from '$app/state';
  import { goto } from '$app/navigation';
  import { isAuthenticated } from '$lib/stores/auth';

  const error = page.error;
  const status = page.status;
  const errorCode = error?.code;
  
  // Get error details based on status and code
  const getErrorDetails = () => {
    if (status === 401) {
      return {
        icon: 'ti ti-lock',
        iconColor: 'text-yellow-600',
        bgColor: 'bg-yellow-100',
        title: 'Authentication Required',
        message: error?.message || 'Please log in to access this resource',
        showLoginButton: true
      };
    }
    
    if (status === 403) {
      return {
        icon: 'ti ti-shield-x',
        iconColor: 'text-red-600',
        bgColor: 'bg-red-100',
        title: 'Access Denied',
        message: error?.message || 'You do not have permission to access this resource',
        showLoginButton: false
      };
    }
    
    if (status === 404) {
      return {
        icon: 'ti ti-file-x',
        iconColor: 'text-gray-600',
        bgColor: 'bg-gray-100',
        title: 'Page Not Found',
        message: error?.message || 'The page you are looking for does not exist',
        showLoginButton: false
      };
    }
    
    // Default error
    return {
      icon: 'ti ti-alert-triangle',
      iconColor: 'text-red-600',
      bgColor: 'bg-red-100',
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
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@tabler/icons-webfont@latest/tabler-icons.min.css">
</svelte:head>

<div class="min-h-screen flex items-center justify-center bg-gray-50 px-4">
  <div class="max-w-lg w-full text-center">
    <!-- Error Icon -->
    <div class="mb-8">
      <div class="mx-auto w-24 h-24 {errorDetails.bgColor} rounded-full flex items-center justify-center">
        <i class="{errorDetails.icon} text-4xl {errorDetails.iconColor}"></i>
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
          class="inline-flex items-center gap-2 px-6 py-3 bg-blue-600 text-white rounded-md hover:bg-blue-700 transition-colors font-medium"
        >
          <i class="ti ti-login text-lg"></i>
          Log In
        </button>
      {/if}
      
      <button 
        onclick={handleGoHome}
        class="inline-flex items-center gap-2 px-6 py-3 bg-white border border-gray-300 text-gray-700 rounded-md hover:bg-gray-50 transition-colors font-medium"
      >
        <i class="ti ti-home text-lg"></i>
        {$isAuthenticated ? 'Dashboard' : 'Home'}
      </button>
    </div>

  </div>
</div>