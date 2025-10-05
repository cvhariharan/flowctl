<script lang="ts">
  import {
    IconX,
    IconCheck,
    IconPlayerPlay,
    IconClockPause,
    IconCircle,
    IconMinus
  } from '@tabler/icons-svelte';
  type StepStatus = 'pending' | 'running' | 'completed' | 'failed' | 'awaiting_approval' | 'cancelled';

  type Step = {
    id: string;
    name: string;
    status: StepStatus;
    description?: string;
  };

  type Props = {
    steps: Step[];
    title?: string;
    orientation?: 'horizontal' | 'vertical';
    size?: 'sm' | 'md' | 'lg';
    showConnectors?: boolean;
  };

  let {
    steps,
    title,
    orientation = 'horizontal',
    size = 'md',
    showConnectors = true
  }: Props = $props();

  const getStepClasses = (status: StepStatus) => {
    const baseClasses = 'rounded-lg border-2 transition-all duration-300';
    const sizeClasses = {
      sm: 'p-3 min-w-[150px]',
      md: 'p-4 min-w-[200px]',
      lg: 'p-6 min-w-[250px]'
    };

    switch (status) {
      case 'failed':
        return `${baseClasses} ${sizeClasses[size]} bg-danger-50 border-danger-500`;
      case 'completed':
        return `${baseClasses} ${sizeClasses[size]} bg-success-50 border-success-500`;
      case 'running':
        return `${baseClasses} ${sizeClasses[size]} bg-primary-50 border-primary-500`;
      case 'awaiting_approval':
        return `${baseClasses} ${sizeClasses[size]} bg-warning-50 border-warning-500`;
      case 'cancelled':
        return `${baseClasses} ${sizeClasses[size]} bg-gray-100 border-gray-400`;
      default:
        return `${baseClasses} ${sizeClasses[size]} bg-gray-50 border-gray-300`;
    }
  };

  const getIconClasses = (status: StepStatus) => {
    const baseClasses = 'rounded-full flex items-center justify-center';
    const sizeClasses = {
      sm: 'w-5 h-5 text-xs',
      md: 'w-6 h-6 text-sm',
      lg: 'w-8 h-8 text-base'
    };

    switch (status) {
      case 'failed':
        return `${baseClasses} ${sizeClasses[size]} bg-danger-500 text-danger-900`;
      case 'completed':
        return `${baseClasses} ${sizeClasses[size]} bg-success-500 text-success-900`;
      case 'running':
        return `${baseClasses} ${sizeClasses[size]} bg-primary-500 text-primary-900 animate-pulse`;
      case 'awaiting_approval':
        return `${baseClasses} ${sizeClasses[size]} bg-warning-500 text-primary-900`;
      case 'cancelled':
        return `${baseClasses} ${sizeClasses[size]} bg-gray-500 text-gray-900`;
      default:
        return `${baseClasses} ${sizeClasses[size]} bg-gray-400 text-gray-900`;
    }
  };

  const getIcon = (status: StepStatus) => {
    switch (status) {
      case 'failed':
        return IconX;
      case 'completed':
        return IconCheck;
      case 'running':
        return IconPlayerPlay;
      case 'awaiting_approval':
        return IconClockPause;
      case 'cancelled':
        return IconCircle;
      default:
        return IconMinus;
    }
  };

  const getStatusMessage = (status: StepStatus): string => {
    switch (status) {
      case 'failed':
        return 'Failed';
      case 'completed':
        return 'Completed';
      case 'running':
        return 'Processing...';
      case 'awaiting_approval':
        return 'Awaiting Approval';
      case 'cancelled':
        return 'Cancelled';
      default:
        return 'Pending';
    }
  };

  const getConnectorClasses = (currentStatus: StepStatus, isLast: boolean) => {
    if (isLast) return 'hidden';

    const baseClasses = orientation === 'horizontal' 
      ? 'absolute right-0 top-1/2 transform translate-x-4 -translate-y-1/2 h-0.5'
      : 'absolute bottom-0 left-1/2 transform -translate-x-1/2 translate-y-4 w-0.5';
    
    const sizeClasses = orientation === 'horizontal' ? 'w-4' : 'h-4';
    
    const colorClasses = currentStatus === 'completed' 
      ? 'bg-success-500' 
      : 'bg-gray-300';

    return `${baseClasses} ${sizeClasses} ${colorClasses}`;
  };

  const getContainerClasses = () => {
    const baseClasses = 'flex';
    const orientationClasses = orientation === 'horizontal' 
      ? 'gap-4 overflow-x-auto pb-2' 
      : 'flex-col gap-4 overflow-y-auto pr-2';

    return `${baseClasses} ${orientationClasses}`;
  };
</script>

<div class="bg-white rounded-lg shadow-sm border border-gray-200 p-6">
  {#if title}
    <h2 class="text-lg font-semibold text-gray-900 mb-4">{title}</h2>
  {/if}
  
  <div class={getContainerClasses()}>
    {#each steps as step, index}
      <div class="{getStepClasses(step.status)} relative">
        <div class="flex justify-between items-center mb-2">
          <span class="font-semibold text-gray-900 {size === 'sm' ? 'text-sm' : size === 'lg' ? 'text-lg' : 'text-base'}">
            {step.name}
          </span>
          <div class={getIconClasses(step.status)}>
            <svelte:component this={getIcon(step.status)} size={16} />
          </div>
        </div>
        
        <p class="text-sm text-gray-600">
          {step.description || getStatusMessage(step.status)}
        </p>

        <!-- Progress connector -->
        {#if showConnectors}
          <div class={getConnectorClasses(step.status, index === steps.length - 1)}></div>
        {/if}
      </div>
    {/each}
  </div>
</div>