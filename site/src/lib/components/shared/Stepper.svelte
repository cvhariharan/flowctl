<script lang="ts">
  import { createEventDispatcher } from 'svelte';

  type Step = {
    id: string;
    label: string;
    description?: string;
    disabled?: boolean;
  };

  type Props = {
    steps: Step[];
    currentStep: string;
    allowBackNavigation?: boolean;
  };

  let {
    steps,
    currentStep = $bindable(),
    allowBackNavigation = true
  }: Props = $props();

  const dispatch = createEventDispatcher<{
    change: { stepId: string; step: Step };
  }>();

  const getCurrentStepIndex = () => {
    return steps.findIndex(step => step.id === currentStep);
  };

  const getStepStatus = (step: Step, index: number) => {
    const currentIndex = getCurrentStepIndex();
    
    if (step.disabled) return 'disabled';
    if (index < currentIndex) return 'completed';
    if (index === currentIndex) return 'current';
    return 'upcoming';
  };

  const getStepClasses = (step: Step, index: number) => {
    const status = getStepStatus(step, index);
    
    switch (status) {
      case 'completed':
        return 'bg-primary-500 text-white border-primary-500';
      case 'current':
        return 'bg-primary-500 text-white border-primary-500';
      case 'disabled':
        return 'bg-gray-100 text-gray-400 border-gray-200 cursor-not-allowed';
      default: // upcoming
        return 'bg-white text-gray-500 border-gray-300';
    }
  };

  const getConnectorClasses = (index: number) => {
    const currentIndex = getCurrentStepIndex();

    if (index < currentIndex) {
      return 'bg-primary-500';
    }
    return 'bg-gray-300';
  };

  const getLabelClasses = (step: Step, index: number) => {
    const status = getStepStatus(step, index);
    
    switch (status) {
      case 'completed':
      case 'current':
        return 'text-gray-900 font-medium';
      case 'disabled':
        return 'text-gray-400';
      default: // upcoming
        return 'text-gray-500';
    }
  };

  const handleStepClick = (step: Step, index: number) => {
    if (step.disabled) return;
    
    const currentIndex = getCurrentStepIndex();
    
    // Allow clicking on current step, completed steps, or next step if back navigation is allowed
    if (index === currentIndex || 
        (allowBackNavigation && index < currentIndex) || 
        index === currentIndex + 1) {
      currentStep = step.id;
      dispatch('change', { stepId: step.id, step });
    }
  };

  const isClickable = (step: Step, index: number) => {
    if (step.disabled) return false;
    
    const currentIndex = getCurrentStepIndex();
    return index === currentIndex || 
           (allowBackNavigation && index < currentIndex) || 
           index === currentIndex + 1;
  };

  const canGoNext = () => {
    const currentIndex = getCurrentStepIndex();
    return currentIndex < steps.length - 1 && !steps[currentIndex + 1]?.disabled;
  };

  const canGoPrevious = () => {
    const currentIndex = getCurrentStepIndex();
    return currentIndex > 0 && allowBackNavigation;
  };

  const goNext = () => {
    if (canGoNext()) {
      const currentIndex = getCurrentStepIndex();
      const nextStep = steps[currentIndex + 1];
      currentStep = nextStep.id;
      dispatch('change', { stepId: nextStep.id, step: nextStep });
    }
  };

  const goPrevious = () => {
    if (canGoPrevious()) {
      const currentIndex = getCurrentStepIndex();
      const prevStep = steps[currentIndex - 1];
      currentStep = prevStep.id;
      dispatch('change', { stepId: prevStep.id, step: prevStep });
    }
  };
</script>

<div class="mb-8">
  <!-- Step Progress Bar -->
  <nav aria-label="Progress" class="mb-6">
    <ol class="flex items-center">
      {#each steps as step, index}
        {@const status = getStepStatus(step, index)}
        {@const clickable = isClickable(step, index)}
        
        <li class="relative {index !== steps.length - 1 ? 'pr-8 sm:pr-20' : ''} flex-1">
          <!-- Step button -->
          <button
            type="button"
            onclick={() => handleStepClick(step, index)}
            disabled={!clickable}
            class="group flex items-center w-full focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2 rounded-lg p-2 {clickable ? 'hover:bg-gray-50' : ''}"
            aria-current={status === 'current' ? 'step' : undefined}
          >
            <!-- Step number/icon -->
            <div class="flex items-center justify-center w-10 h-10 rounded-full border-2 transition-colors {getStepClasses(step, index)}">
              {#if status === 'completed'}
                <!-- Checkmark icon -->
                <svg class="w-6 h-6" fill="currentColor" viewBox="0 0 20 20">
                  <path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd" />
                </svg>
              {:else}
                <span class="text-sm font-medium">{index + 1}</span>
              {/if}
            </div>
            
            <!-- Step label and description -->
            <div class="ml-4 text-left min-w-0 flex-1">
              <div class="text-sm font-medium {getLabelClasses(step, index)}">
                {step.label}
              </div>
              {#if step.description}
                <div class="text-xs text-gray-500 mt-0.5">
                  {step.description}
                </div>
              {/if}
            </div>
          </button>
          
          <!-- Connector line -->
          {#if index !== steps.length - 1}
            <div class="absolute top-5 right-0 w-8 sm:w-20 h-0.5 {getConnectorClasses(index)} transition-colors"></div>
          {/if}
        </li>
      {/each}
    </ol>
  </nav>

  <!-- Navigation buttons -->
  <div class="flex justify-between items-center">
    <button
      type="button"
      onclick={goPrevious}
      disabled={!canGoPrevious()}
      class="inline-flex items-center px-4 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-primary-500 disabled:opacity-50 disabled:cursor-not-allowed"
    >
      <svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7" />
      </svg>
      Previous
    </button>

    <span class="text-sm text-gray-500">
      Step {getCurrentStepIndex() + 1} of {steps.length}
    </span>

    <button
      type="button"
      onclick={goNext}
      disabled={!canGoNext()}
      class="inline-flex items-center px-4 py-2 text-sm font-medium text-white bg-primary-500 border border-transparent rounded-md hover:bg-primary-600 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-primary-500 disabled:opacity-50 disabled:cursor-not-allowed"
    >
      Next
      <svg class="w-4 h-4 ml-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
      </svg>
    </button>
  </div>
</div>