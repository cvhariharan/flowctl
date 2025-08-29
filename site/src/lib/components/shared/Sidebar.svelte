<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/state';
  import { apiClient } from '$lib/apiClient';
  import { handleInlineError } from '$lib/utils/errorHandling';
  import { currentUser } from '$lib/stores/auth';
  import type { NamespaceResp, Namespace } from '$lib/types';
  import { DEFAULT_PAGE_SIZE } from '$lib/constants';
  import { setContext } from 'svelte';
  import { permissionChecker, type ResourcePermissions } from '$lib/utils/permissions';
  import {
    IconChevronDown,
    IconGridDots,
    IconServer,
    IconKey,
    IconUsers,
    IconCircleCheck,
    IconClock,
    IconSettings
  } from '@tabler/icons-svelte';

  let { namespace }: {namespace: string} = $props();

  let namespaceDropdownOpen = $state(false);
  let namespaces = $state<Namespace[]>([]);
  let currentNamespace = $state(page.params.namespace || namespace);
  let currentNamespaceId = $state<string>('');
  let permissions = $state<{
    flows: ResourcePermissions;
    nodes: ResourcePermissions;
    credentials: ResourcePermissions;
    members: ResourcePermissions;
    approvals: ResourcePermissions;
    history: ResourcePermissions;
  }>({
    flows: { canCreate: false, canUpdate: false, canDelete: false, canRead: false },
    nodes: { canCreate: false, canUpdate: false, canDelete: false, canRead: false },
    credentials: { canCreate: false, canUpdate: false, canDelete: false, canRead: false },
    members: { canCreate: false, canUpdate: false, canDelete: false, canRead: false },
    approvals: { canCreate: false, canUpdate: false, canDelete: false, canRead: false },
    history: { canCreate: false, canUpdate: false, canDelete: false, canRead: false }
  });

  const isActiveLink = (section: string): boolean => {
    const currentPath = page.url.pathname;
    
    if (section === 'flows') {
      return currentPath.includes('/flows');
    } else if (section === 'nodes') {
      return currentPath.includes('/nodes');
    } else if (section === 'credentials') {
      return currentPath.includes('/credentials');
    } else if (section === 'members') {
      return currentPath.includes('/members');
    } else if (section === 'approvals') {
      return currentPath.includes('/approvals');
    } else if (section === 'history') {
      return currentPath.includes('/history');
    } else if (section === 'settings') {
      return currentPath.includes('/settings');
    }
    
    return false;
  };

  const fetchNamespaces = async () => {
    try {
      const data = await apiClient.namespaces.list({ count_per_page: DEFAULT_PAGE_SIZE });
      namespaces = data.namespaces || [];
      
      // Set current namespace ID
      const currentNs = namespaces.find(ns => ns.name === namespace);
      if (currentNs) {
        currentNamespaceId = currentNs.id;
      } else if (namespaces.length > 0) {
        // If namespace not found, use first available namespace
        currentNamespaceId = namespaces[0].id;
      }
    } catch (error) {
      handleInlineError(error, 'Unable to Load Namespaces');
      namespaces = [];
    }
  };

  const checkAllPermissions = async () => {
    if (!$currentUser || !currentNamespaceId) return;
    
    const resourceMappings = {
      flows: 'flow',
      nodes: 'node', 
      credentials: 'credential',
      members: 'member',
      approvals: 'approval',
      history: 'execution'
    };
    
    try {
      const permissionPromises = Object.entries(resourceMappings).map(async ([frontendKey, backendResource]) => {
        const perms = await permissionChecker($currentUser, backendResource, currentNamespaceId, ['view']);
        return { frontendKey, perms };
      });
      
      const results = await Promise.all(permissionPromises);
      
      results.forEach(({ frontendKey, perms }) => {
        permissions[frontendKey as keyof typeof permissions] = perms;
      });
    } catch (error) {
      handleInlineError(error, 'Unable to Check Sidebar Permissions');
    }
  };

  const selectNamespace = (selectedNamespace: Namespace) => {
    namespaceDropdownOpen = false;
    
    // Don't navigate if already on the same namespace
    if (selectedNamespace.name === namespace) {
      return;
    }
    
    // Force a full page reload by using window.location
    window.location.href = `/view/${selectedNamespace.name}/flows`;
  };

  // Set initial context
  setContext('namespace', namespace);

  onMount(() => {
    fetchNamespaces();
    checkAllPermissions();
  });

  // Update currentNamespace when namespace prop changes
  $effect(() => {
    currentNamespace = page.params.namespace || namespace;
  });

  // Re-check permissions when currentUser or namespace changes
  $effect(() => {
    if ($currentUser && currentNamespaceId) {
      checkAllPermissions();
    }
  });
</script>

<!-- Sidebar Navigation -->
<div class="w-60 bg-slate-800 flex flex-col">
  <!-- Logo -->
  <div class="flex items-center px-6 py-6">
    <div class="w-8 h-8 bg-blue-500 rounded-lg flex items-center justify-center">
      <span class="text-white font-bold text-lg">F</span>
    </div>
    <span class="ml-3 text-white font-semibold text-xl">Flowctl</span>
  </div>

  <!-- Namespace Dropdown -->
  <div class="px-4 mb-4">
    <div class="relative">
      <label for="namespace-dropdown" class="block text-xs font-medium text-gray-400 mb-1">Namespace</label>
      <button 
        type="button"
        id="namespace-dropdown"
        onclick={() => namespaceDropdownOpen = !namespaceDropdownOpen}
        class="w-full flex items-center justify-between px-3 py-2 text-sm font-medium text-white bg-slate-700 rounded-lg hover:bg-slate-600 transition-colors"
      >
        <span>{currentNamespace || 'Select namespace'}</span>
        <IconChevronDown class="text-base text-gray-400 transition-transform {namespaceDropdownOpen ? 'rotate-180' : ''}" size={16} />
      </button>
      
      <!-- Dropdown Menu -->
      {#if namespaceDropdownOpen}
        <div 
          class="absolute z-50 w-full mt-1 bg-slate-700 rounded-lg shadow-lg ring-1 ring-black ring-opacity-5 max-h-48 overflow-y-auto"
          role="menu"
        >
          <div class="py-1">
            {#each namespaces as ns (ns.id)}
              <button 
                type="button"
                onclick={() => selectNamespace(ns)}
                class="w-full text-left px-3 py-2 text-sm text-white hover:bg-slate-600 transition-colors"
                class:bg-slate-600={ns.name === namespace}
              >
                {ns.name}
              </button>
            {/each}
            {#if namespaces.length === 0}
              <div class="px-3 py-2 text-sm text-gray-400">
                No namespaces available
              </div>
            {/if}
          </div>
        </div>
      {/if}
    </div>
  </div>

  <!-- Navigation -->
  <nav class="flex-1 px-4 space-y-1">
    {#if permissions.flows.canRead}
      <a 
        href="/view/{namespace}/flows" 
        class="flex items-center px-4 py-3 text-sm font-medium rounded-lg transition-colors"
        class:bg-blue-600={isActiveLink('flows')}
        class:text-white={isActiveLink('flows')}
        class:text-gray-300={!isActiveLink('flows')}
        class:hover:bg-slate-700={!isActiveLink('flows')}
        class:hover:text-white={!isActiveLink('flows')}
      >
        <IconGridDots class="text-xl mr-3 flex-shrink-0" size={20} />
        Flows
      </a>
    {/if}
    {#if permissions.nodes.canRead}
      <a 
        href="/view/{namespace}/nodes" 
        class="flex items-center px-4 py-3 text-sm font-medium rounded-lg transition-colors"
        class:bg-blue-600={isActiveLink('nodes')}
        class:text-white={isActiveLink('nodes')}
        class:text-gray-300={!isActiveLink('nodes')}
        class:hover:bg-slate-700={!isActiveLink('nodes')}
        class:hover:text-white={!isActiveLink('nodes')}
      >
        <IconServer class="text-xl mr-3 flex-shrink-0" size={20} />
        Nodes
      </a>
    {/if}
    {#if permissions.credentials.canRead}
      <a 
        href="/view/{namespace}/credentials" 
        class="flex items-center px-4 py-3 text-sm font-medium rounded-lg transition-colors"
        class:bg-blue-600={isActiveLink('credentials')}
        class:text-white={isActiveLink('credentials')}
        class:text-gray-300={!isActiveLink('credentials')}
        class:hover:bg-slate-700={!isActiveLink('credentials')}
        class:hover:text-white={!isActiveLink('credentials')}
      >
        <IconKey class="text-xl mr-3 flex-shrink-0" size={20} />
        Credentials
      </a>
    {/if}
    {#if permissions.members.canRead}
      <a 
        href="/view/{namespace}/members" 
        class="flex items-center px-4 py-3 text-sm font-medium rounded-lg transition-colors"
        class:bg-blue-600={isActiveLink('members')}
        class:text-white={isActiveLink('members')}
        class:text-gray-300={!isActiveLink('members')}
        class:hover:bg-slate-700={!isActiveLink('members')}
        class:hover:text-white={!isActiveLink('members')}
      >
        <IconUsers class="text-xl mr-3 flex-shrink-0" size={20} />
        Members
      </a>
    {/if}
    {#if permissions.approvals.canRead}
      <a 
        href="/view/{namespace}/approvals" 
        class="flex items-center px-4 py-3 text-sm font-medium rounded-lg transition-colors"
        class:bg-blue-600={isActiveLink('approvals')}
        class:text-white={isActiveLink('approvals')}
        class:text-gray-300={!isActiveLink('approvals')}
        class:hover:bg-slate-700={!isActiveLink('approvals')}
        class:hover:text-white={!isActiveLink('approvals')}
      >
        <IconCircleCheck class="text-xl mr-3 flex-shrink-0" size={20} />
        Approvals
      </a>
    {/if}
    {#if permissions.history.canRead}
      <a 
        href="/view/{namespace}/history" 
        class="flex items-center px-4 py-3 text-sm font-medium rounded-lg transition-colors"
        class:bg-blue-600={isActiveLink('history')}
        class:text-white={isActiveLink('history')}
        class:text-gray-300={!isActiveLink('history')}
        class:hover:bg-slate-700={!isActiveLink('history')}
        class:hover:text-white={!isActiveLink('history')}
      >
        <IconClock class="text-xl mr-3 flex-shrink-0" size={20} />
        History
      </a>
    {/if}
    <!-- Settings (only show for superusers) -->
    {#if $currentUser && $currentUser.role === 'superuser'}
      <a 
        href="/settings" 
        class="flex items-center px-4 py-3 text-sm font-medium rounded-lg transition-colors"
        class:bg-blue-600={isActiveLink('settings')}
        class:text-white={isActiveLink('settings')}
        class:text-gray-300={!isActiveLink('settings')}
        class:hover:bg-slate-700={!isActiveLink('settings')}
        class:hover:text-white={!isActiveLink('settings')}
      >
        <IconSettings class="text-xl mr-3 flex-shrink-0" size={20} />
        Settings
      </a>
    {/if}
  </nav>
</div>