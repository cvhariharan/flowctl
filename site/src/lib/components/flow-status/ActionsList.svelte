<script lang="ts">
  import {
    IconX,
    IconCheck,
    IconPlayerPlay,
    IconClockPause,
    IconCircle,
    IconMinus,
    IconSearch
  } from '@tabler/icons-svelte';

  type StepStatus = 'pending' | 'running' | 'completed' | 'failed' | 'awaiting_approval' | 'cancelled';

  type Action = {
    id: string;
    name: string;
    status: StepStatus;
  };

  type Props = {
    actions: Action[];
    selectedActionId?: string;
    onActionSelect: (actionId: string) => void;
  };

  let {
    actions,
    selectedActionId = $bindable(),
    onActionSelect
  }: Props = $props();

  let searchQuery = $state('');

  const filteredActions = $derived(
    actions.filter(action =>
      action.name.toLowerCase().includes(searchQuery.toLowerCase())
    )
  );

  const getStatusClasses = (status: StepStatus) => {
    switch (status) {
      case 'failed':
        return 'bg-danger-50 text-danger-700';
      case 'completed':
        return 'bg-success-50 text-success-700';
      case 'running':
        return 'bg-primary-50 text-primary-700';
      case 'awaiting_approval':
        return 'bg-warning-50 text-warning-700';
      case 'cancelled':
        return 'bg-gray-100 text-gray-700';
      default:
        return 'bg-gray-50 text-gray-600';
    }
  };

  const getIconClasses = (status: StepStatus) => {
    switch (status) {
      case 'failed':
        return 'bg-danger-500 text-white';
      case 'completed':
        return 'bg-success-500 text-white';
      case 'running':
        return 'bg-primary-500 text-white animate-pulse';
      case 'awaiting_approval':
        return 'bg-warning-500 text-white';
      case 'cancelled':
        return 'bg-gray-500 text-white';
      default:
        return 'bg-gray-400 text-white';
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

  const handleActionClick = (actionId: string) => {
    onActionSelect(actionId);
  };
</script>

<div class="flex flex-col h-full bg-white rounded-lg border border-gray-300 overflow-hidden">
  <!-- Header with Search -->
  <div class="sticky top-0 bg-white border-b border-gray-300 px-6 py-5 space-y-4 z-10">
    <h2 class="text-base font-semibold text-gray-900">Actions</h2>

    <!-- Search Input -->
    <div class="relative">
      <div class="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
        <IconSearch size={18} class="text-gray-400" />
      </div>
      <input
        type="text"
        bind:value={searchQuery}
        placeholder="Search actions..."
        class="w-full pl-10 pr-4 py-2 text-sm border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-primary-500"
      />
    </div>
  </div>

  <!-- Actions List -->
  <div class="flex-1 overflow-y-auto p-4">
    {#if filteredActions.length === 0}
      <div class="text-center py-8 text-gray-500 text-sm">
        {searchQuery ? 'No actions found' : 'No actions available'}
      </div>
    {:else}
      <div class="space-y-3">
        {#each filteredActions as action (action.id)}
          <button
            type="button"
            onclick={() => handleActionClick(action.id)}
            class="w-full text-left p-4 rounded-lg border transition-all duration-200 cursor-pointer {getStatusClasses(action.status)}"
            class:border-gray-100={selectedActionId !== action.id}
            class:border-2={selectedActionId !== action.id}
          >
            <div class="flex items-center justify-between gap-3">
              <div class="flex-1 min-w-0">
                <p class="font-medium text-sm truncate">{action.name}</p>
              </div>
              <div class="flex-shrink-0">
                <div class="rounded-full p-1.5 {getIconClasses(action.status)}">
                  <svelte:component this={getIcon(action.status)} size={16} />
                </div>
              </div>
            </div>
          </button>
        {/each}
      </div>
    {/if}
  </div>
</div>
