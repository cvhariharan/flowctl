<script lang="ts">
  import { createEventDispatcher } from 'svelte';

  type Tab = {
    id: string;
    label: string;
    badge?: string | number;
    disabled?: boolean;
  };

  type Props = {
    tabs: Tab[];
    activeTab: string;
    variant?: 'default' | 'pills';
    size?: 'sm' | 'md' | 'lg';
  };

  let {
    tabs,
    activeTab = $bindable(),
    variant = 'default',
    size = 'md'
  }: Props = $props();

  const dispatch = createEventDispatcher<{
    change: { tabId: string; tab: Tab };
  }>();

  const getTabClasses = (tab: Tab, isActive: boolean) => {
    const baseClasses = 'font-medium transition-colors focus:outline-none border-b-2';
    const sizeClasses = {
      sm: 'py-2 px-4 text-xs',
      md: 'py-4 px-1 text-sm',
      lg: 'py-4 px-8 text-base'
    };

    if (variant === 'pills') {
      const pillBaseClasses = 'font-medium transition-colors rounded-lg focus:outline-none';
      const pillSizeClasses = {
        sm: 'py-1 px-3 text-xs',
        md: 'py-2 px-4 text-sm',
        lg: 'py-3 px-6 text-base'
      };

      if (tab.disabled) {
        return `${pillBaseClasses} ${pillSizeClasses[size]} bg-gray-100 text-gray-400 cursor-not-allowed`;
      }

      if (isActive) {
        return `${pillBaseClasses} ${pillSizeClasses[size]} bg-primary-500 text-white shadow-sm`;
      }

      return `${pillBaseClasses} ${pillSizeClasses[size]} text-gray-600 hover:text-gray-900 hover:bg-gray-100`;
    }

    // Default variant
    if (tab.disabled) {
      return `${baseClasses} ${sizeClasses[size]} border-transparent text-gray-400 cursor-not-allowed`;
    }

    if (isActive) {
      return `${baseClasses} ${sizeClasses[size]} border-primary-600 text-primary-600`;
    }

    return `${baseClasses} ${sizeClasses[size]} border-transparent text-gray-500 hover:text-gray-700`;
  };

  const getContainerClasses = () => {
    if (variant === 'pills') {
      return 'flex space-x-1 p-1 bg-gray-100 rounded-lg';
    }
    return 'flex space-x-8 px-6';
  };

  const handleTabClick = (tab: Tab) => {
    if (tab.disabled) return;

    activeTab = tab.id;
    dispatch('change', { tabId: tab.id, tab });
  };
</script>

<nav class={getContainerClasses()} aria-label="Tabs">
  {#each tabs as tab}
    {@const isActive = activeTab === tab.id}
    <button
      type="button"
      onclick={() => handleTabClick(tab)}
      disabled={tab.disabled}
      class="{getTabClasses(tab, isActive)} cursor-pointer"
      aria-current={isActive ? 'page' : undefined}
    >
      <span class="whitespace-nowrap">
        {tab.label}
        {#if tab.badge}
          <span class="ml-2 inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium {isActive && variant !== 'pills' ? 'bg-primary-100 text-primary-900' : variant === 'pills' && isActive ? 'bg-primary-600 text-white' : 'bg-gray-100 text-gray-800'}">
            {tab.badge}
          </span>
        {/if}
      </span>
    </button>
  {/each}
</nav>