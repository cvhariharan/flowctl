import { writable } from 'svelte/store';

export interface Notification {
  id: string;
  type: 'success' | 'error' | 'warning' | 'info';
  title: string;
  message: string;
  duration?: number; // Duration in ms, 0 for persistent
  dismissible?: boolean;
}

function createNotificationStore() {
  const { subscribe, update } = writable<Notification[]>([]);

  const store = {
    subscribe,
    add: (notification: Omit<Notification, 'id'>) => {
      const id = crypto.randomUUID();
      const newNotification: Notification = {
        id,
        duration: 5000, // Default 5 seconds
        dismissible: true,
        ...notification,
      };

      update(notifications => [...notifications, newNotification]);

      // Auto-remove after duration if specified
      if (newNotification.duration && newNotification.duration > 0) {
        setTimeout(() => {
          update(notifications => notifications.filter(n => n.id !== id));
        }, newNotification.duration);
      }

      return id;
    },
    remove: (id: string) => {
      update(notifications => notifications.filter(n => n.id !== id));
    },
    clear: () => {
      update(() => []);
    }
  };

  return {
    ...store,
    // Helper methods for different notification types
    success: (title: string, message: string, options?: Partial<Notification>) => {
      return store.add({ type: 'success', title, message, ...options });
    },
    error: (title: string, message: string, options?: Partial<Notification>) => {
      return store.add({ type: 'error', title, message, duration: 0, ...options });
    },
    warning: (title: string, message: string, options?: Partial<Notification>) => {
      return store.add({ type: 'warning', title, message, ...options });
    },
    info: (title: string, message: string, options?: Partial<Notification>) => {
      return store.add({ type: 'info', title, message, ...options });
    }
  };
}

export const notifications = createNotificationStore();