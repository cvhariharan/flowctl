<script lang="ts">
  type AlertType = 'info' | 'success' | 'warning' | 'error';
  
  type Props = {
    type: AlertType;
    title?: string;
    message: string;
    showIcon?: boolean;
    dismissible?: boolean;
    onDismiss?: () => void;
    actions?: Array<{
      label: string;
      href?: string;
      onClick?: () => void;
      primary?: boolean;
    }>;
  };

  let {
    type,
    title,
    message,
    showIcon = true,
    dismissible = false,
    onDismiss,
    actions = []
  }: Props = $props();

  const getAlertClasses = (alertType: AlertType) => {
    const baseClasses = 'rounded-lg border p-6';

    switch (alertType) {
      case 'success':
        return `${baseClasses} bg-success-50 border-success-100`;
      case 'warning':
        return `${baseClasses} bg-warning-50 border-warning-100`;
      case 'error':
        return `${baseClasses} bg-danger-50 border-danger-100`;
      default:
        return `${baseClasses} bg-info-50 border-info-100`;
    }
  };

  const getIconClasses = (alertType: AlertType) => {
    const baseClasses = 'h-5 w-5';

    switch (alertType) {
      case 'success':
        return `${baseClasses} text-success-500`;
      case 'warning':
        return `${baseClasses} text-warning-500`;
      case 'error':
        return `${baseClasses} text-danger-500`;
      default:
        return `${baseClasses} text-info-500`;
    }
  };

  const getTextClasses = (alertType: AlertType) => {
    switch (alertType) {
      case 'success':
        return { title: 'text-success-900', message: 'text-success-800' };
      case 'warning':
        return { title: 'text-warning-900', message: 'text-warning-800' };
      case 'error':
        return { title: 'text-danger-900', message: 'text-danger-800' };
      default:
        return { title: 'text-info-900', message: 'text-info-800' };
    }
  };

  const getIcon = (alertType: AlertType) => {
    switch (alertType) {
      case 'success':
        return 'M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z';
      case 'warning':
        return 'M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-.835-1.964-.835-2.732 0L2.27 16c-.77.835.19 2.5 1.73 2.5z';
      case 'error':
        return 'M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z';
      default:
        return 'M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z';
    }
  };

  const getButtonClasses = (alertType: AlertType, primary: boolean = false) => {
    const baseClasses = 'inline-flex items-center px-4 py-2 text-sm font-medium rounded-md transition-colors';

    if (primary) {
      switch (alertType) {
        case 'success':
          return `${baseClasses} bg-success-500 text-white hover:bg-success-600`;
        case 'warning':
          return `${baseClasses} bg-warning-500 text-white hover:bg-warning-600`;
        case 'error':
          return `${baseClasses} bg-danger-500 text-white hover:bg-danger-600`;
        default:
          return `${baseClasses} bg-info-500 text-white hover:bg-info-600`;
      }
    } else {
      switch (alertType) {
        case 'success':
          return `${baseClasses} bg-success-100 text-success-900 hover:bg-success-200`;
        case 'warning':
          return `${baseClasses} bg-warning-100 text-warning-900 hover:bg-warning-200`;
        case 'error':
          return `${baseClasses} bg-danger-100 text-danger-900 hover:bg-danger-200`;
        default:
          return `${baseClasses} bg-info-100 text-info-900 hover:bg-info-200`;
      }
    }
  };

  const handleDismiss = () => {
    if (onDismiss) {
      onDismiss();
    }
  };

  const handleActionClick = (action: NonNullable<Props['actions']>[0]) => {
    if (action.onClick) {
      action.onClick();
    }
  };

  const textClasses = getTextClasses(type);
</script>

<div class={getAlertClasses(type)}>
  <div class="flex {actions.length > 0 ? 'justify-between items-start' : ''}">
    <div class="flex">
      {#if showIcon}
        <div class="flex-shrink-0">
          <svg class={getIconClasses(type)} fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d={getIcon(type)}/>
          </svg>
        </div>
      {/if}
      <div class="{showIcon ? 'ml-3' : ''}">
        {#if title}
          <h3 class="text-sm font-medium {textClasses.title}">{title}</h3>
        {/if}
        <p class="text-sm {textClasses.message} {title ? 'mt-1' : ''}">{message}</p>
      </div>
    </div>
    
    {#if actions.length > 0}
      <div class="flex items-center gap-2 {showIcon ? 'ml-4' : ''}">
        {#each actions as action}
          {#if action.href}
            <a href={action.href} class={getButtonClasses(type, action.primary)}>
              {action.label}
              <svg class="ml-2 -mr-1 h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7"/>
              </svg>
            </a>
          {:else}
            <button onclick={() => handleActionClick(action)} class={getButtonClasses(type, action.primary)}>
              {action.label}
            </button>
          {/if}
        {/each}
      </div>
    {/if}

    {#if dismissible}
      <button onclick={handleDismiss} class="ml-4 inline-flex text-gray-400 hover:text-gray-600">
        <svg class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/>
        </svg>
      </button>
    {/if}
  </div>
</div>