import { error } from '@sveltejs/kit';
import { goto } from '$app/navigation';
import { notifications } from '$lib/stores/notifications';
import { ApiError } from '$lib/apiClient';

export interface ErrorHandlingOptions {
  showNotification?: boolean;
  redirectOnAuth?: boolean;
  customTitle?: string;
  throwError?: boolean;
}

/**
 * Universal error handler - handles all API errors consistently
 */
export function handleError(err: unknown, options: ErrorHandlingOptions = {}) {
  const {
    showNotification = true,
    redirectOnAuth = true,
    customTitle,
    throwError = false
  } = options;

  if (err instanceof ApiError) {
    const { status, data } = err;
    
    // Handle authentication errors - redirect to login
    if (status === 401 && redirectOnAuth) {
      goto('/login');
      return;
    }
    
    // Show notification using API response data
    if (showNotification && data) {
      const title = customTitle || formatErrorCode(data.code) || 'Error';
      const message = formatErrorMessage(data);
      
      notifications.error(title, message, {
        duration: isValidationError(data.code) ? 8000 : 5000
      });
    }
    
    // Optionally throw SvelteKit error (for load functions)
    if (throwError) {
      throw error(status, {
        message: data?.error || 'An error occurred',
        code: data?.code,
        details: data?.details
      });
    }
    
    return;
  }
  
  // Handle unknown errors
  console.error('Unknown error:', err);
  
  if (showNotification) {
    notifications.error(
      customTitle || 'Unexpected Error',
      'An unexpected error occurred'
    );
  }
  
  if (throwError) {
    throw error(500, {
      message: 'An unexpected error occurred',
      code: 'UNKNOWN_ERROR'
    });
  }
}

/**
 * Format error code to be more human-readable as title
 * e.g., "VALIDATION_FAILED" -> "Validation Failed"
 */
function formatErrorCode(code?: string): string | undefined {
  if (!code) return undefined;
  
  return code
    .toLowerCase()
    .split('_')
    .map(word => word.charAt(0).toUpperCase() + word.slice(1))
    .join(' ');
}

/**
 * Format error message from API response, handling field validation details
 */
function formatErrorMessage(data: { error: string; details?: any }): string {
  // Handle single validation error object
  if (data.details && data.details.field && data.details.error) {
    return `${data.details.field}: ${data.details.error}`;
  }
  
  return data.error;
}

/**
 * Check if error code is a validation error that should show longer
 */
function isValidationError(code?: string): boolean {
  return code === 'VALIDATION_FAILED' || 
         code === 'REQUIRED_FIELD_MISSING' || 
         code === 'INVALID_INPUT';
}

/**
 * Shows a success notification for API operations
 */
export function showSuccess(title: string, message: string) {
  notifications.success(title, message);
}

export function showInfo(title: string, message: string) {
  notifications.info(title, message);
}

export function showWarning(title: string, message: string) {
  notifications.warning(title, message);
}


/**
 * For load functions - throws SvelteKit errors for error pages
 */
export function handleLoadError(err: unknown, customTitle?: string) {
  return handleError(err, {
    showNotification: false,
    throwError: true,
    customTitle
  });
}

/**
 * For page actions - shows notifications and handles auth redirects
 */
export function handlePageError(err: unknown, customTitle?: string) {
  return handleError(err, {
    showNotification: true,
    redirectOnAuth: true,
    customTitle
  });
}

/**
 * For inline errors - shows notifications but no auth redirect (like modals)
 */
export function handleInlineError(err: unknown, customTitle?: string) {
  return handleError(err, {
    showNotification: true,
    redirectOnAuth: false,
    customTitle
  });
}