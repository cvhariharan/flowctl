<script lang="ts">
  import { notifications, type Notification } from '$lib/stores/notifications';
  import { fly, scale } from 'svelte/transition';
  import { flip } from 'svelte/animate';

  const handleDismiss = (id: string) => {
    notifications.remove(id);
  };

  const getIconAndColors = (type: Notification['type']) => {
    switch (type) {
      case 'success':
        return {
          icon: 'ti ti-check-circle',
          bgColor: 'bg-green-50',
          borderColor: 'border-green-200',
          iconColor: 'text-green-400',
          titleColor: 'text-green-800',
          messageColor: 'text-green-700',
          buttonColor: 'text-green-500 hover:bg-green-100'
        };
      case 'error':
        return {
          icon: 'ti ti-alert-circle',
          bgColor: 'bg-red-50',
          borderColor: 'border-red-200',
          iconColor: 'text-red-400',
          titleColor: 'text-red-800',
          messageColor: 'text-red-700',
          buttonColor: 'text-red-500 hover:bg-red-100'
        };
      case 'warning':
        return {
          icon: 'ti ti-alert-triangle',
          bgColor: 'bg-yellow-50',
          borderColor: 'border-yellow-200',
          iconColor: 'text-yellow-400',
          titleColor: 'text-yellow-800',
          messageColor: 'text-yellow-700',
          buttonColor: 'text-yellow-500 hover:bg-yellow-100'
        };
      case 'info':
      default:
        return {
          icon: 'ti ti-info-circle',
          bgColor: 'bg-blue-50',
          borderColor: 'border-blue-200',
          iconColor: 'text-blue-400',
          titleColor: 'text-blue-800',
          messageColor: 'text-blue-700',
          buttonColor: 'text-blue-500 hover:bg-blue-100'
        };
    }
  };
</script>

<div class="fixed top-4 right-4 z-50 space-y-3 max-w-sm w-full">
  {#each $notifications as notification (notification.id)}
    {@const styles = getIconAndColors(notification.type)}
    <div
      class="rounded-lg border p-4 shadow-lg {styles.bgColor} {styles.borderColor}"
      in:fly={{ x: 300, duration: 300 }}
      out:scale={{ duration: 200 }}
      animate:flip={{ duration: 200 }}
    >
      <div class="flex">
        <i class="{styles.icon} {styles.iconColor} mt-0.5 text-lg"></i>
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
            class="ml-auto -mx-1.5 -my-1.5 rounded-lg focus:ring-2 p-1.5 inline-flex h-8 w-8 {styles.buttonColor}"
          >
            <span class="sr-only">Dismiss</span>
            <i class="ti ti-x text-sm"></i>
          </button>
        {/if}
      </div>
    </div>
  {/each}
</div>