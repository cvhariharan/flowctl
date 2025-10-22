<script lang="ts">
  import { notifications, type Notification } from '$lib/stores/notifications';
  import { fly, scale } from 'svelte/transition';
  import { flip } from 'svelte/animate';
  import {
    IconCircleCheck,
    IconAlertCircle,
    IconAlertTriangle,
    IconInfoCircle,
    IconX
  } from '@tabler/icons-svelte';

  const handleDismiss = (id: string) => {
    notifications.remove(id);
  };

  const getIconAndColors = (type: Notification['type']) => {
    switch (type) {
      case 'success':
        return {
          IconComponent: IconCircleCheck,
          bgColor: 'bg-success-50',
          borderColor: 'border-success-100',
          iconColor: 'text-success-500',
          titleColor: 'text-success-900',
          messageColor: 'text-success-800',
          buttonColor: 'text-success-500 hover:bg-success-100'
        };
      case 'error':
        return {
          IconComponent: IconAlertCircle,
          bgColor: 'bg-danger-50',
          borderColor: 'border-danger-100',
          iconColor: 'text-danger-500',
          titleColor: 'text-danger-900',
          messageColor: 'text-danger-800',
          buttonColor: 'text-danger-500 hover:bg-danger-100'
        };
      case 'warning':
        return {
          IconComponent: IconAlertTriangle,
          bgColor: 'bg-warning-50',
          borderColor: 'border-warning-100',
          iconColor: 'text-warning-500',
          titleColor: 'text-warning-900',
          messageColor: 'text-warning-800',
          buttonColor: 'text-warning-500 hover:bg-warning-100'
        };
      case 'info':
      default:
        return {
          IconComponent: IconInfoCircle,
          bgColor: 'bg-info-50',
          borderColor: 'border-info-100',
          iconColor: 'text-info-500',
          titleColor: 'text-info-900',
          messageColor: 'text-info-800',
          buttonColor: 'text-info-500 hover:bg-info-100'
        };
    }
  };
</script>

<div class="fixed top-4 right-4 z-50 space-y-3 max-w-sm w-full" role="region" aria-label="Notifications">
  {#each $notifications as notification (notification.id)}
    {@const styles = getIconAndColors(notification.type)}
    <div
      class="rounded-lg border p-4 shadow-lg {styles.bgColor} {styles.borderColor}"
      role="alert"
      aria-live={notification.type === 'error' ? 'assertive' : 'polite'}
      in:fly={{ x: 300, duration: 300 }}
      out:scale={{ duration: 200 }}
      animate:flip={{ duration: 200 }}
    >
      <div class="flex">
        <styles.IconComponent class="{styles.iconColor} mt-0.5" size={18} aria-hidden="true" />
        <div class="ml-3 flex-1">
          <h3 class="text-sm font-medium {styles.titleColor}">
            {notification.title}
          </h3>
          <p class="mt-1 text-sm {styles.messageColor}">
            {notification.message}
          </p>
        </div>
        {#if notification.dismissible}
          <button
            onclick={() => handleDismiss(notification.id)}
            class="ml-auto -mx-1.5 -my-1.5 rounded-lg focus:ring-2 p-1.5 inline-flex h-8 w-8 cursor-pointer {styles.buttonColor}"
            aria-label="Dismiss notification"
          >
            <span class="sr-only">Dismiss</span>
            <IconX size={16} aria-hidden="true" />
          </button>
        {/if}
      </div>
    </div>
  {/each}
</div>