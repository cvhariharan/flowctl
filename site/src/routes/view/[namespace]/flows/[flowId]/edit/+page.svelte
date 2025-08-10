<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { apiClient } from '$lib/apiClient.js';
  import Header from '$lib/components/shared/Header.svelte';
  import FlowMetadata from '$lib/components/flow-create/FlowMetadata.svelte';
  import FlowInputs from '$lib/components/flow-create/FlowInputs.svelte';
  import FlowActions from '$lib/components/flow-create/FlowActions.svelte';
  import ValidationModal from '$lib/components/flow-create/ValidationModal.svelte';
  import type { PageData } from './$types';
  import type { FlowUpdateReq, FlowInputReq, FlowActionReq } from '$lib/types.js';
  import { goto } from '$app/navigation';
  import { handleInlineError, showSuccess } from '$lib/utils/errorHandling';

  let { data }: { data: PageData } = $props();
  const namespace = $page.params.namespace as string;
  const flowId = $page.params.flowId as string;

  // Flow state
  let flow = $state({
    metadata: {
      id: '',
      name: '',
      description: '',
      namespace: namespace
    },
    inputs: [] as any[],
    actions: [] as any[]
  });

  // Modal states
  let showValidation = $state(false);
  let validationResult = $state({
    success: false,
    errors: [] as string[]
  });

  // Loading states
  let loading = $state(true);
  let saving = $state(false);
  const availableExecutors = data.availableExecutors;
  
  // Executor configs for actions
  let executorConfigs = $state({} as Record<string, any>);

  onMount(async () => {
    await loadFlowConfig();
  });

  async function loadExecutorConfigs(actions: any[]) {
    const executorTypes = [...new Set(actions.map(action => action.executor).filter(Boolean))];
    
    for (const executor of executorTypes) {
      try {
        const config = await apiClient.executors.getConfig(executor);
        
        // Handle both direct schema and $ref-based schemas
        if (config.$defs && config.$ref) {
          const refPath = config.$ref.replace('#/$defs/', '');
          const schema = config.$defs[refPath];
          executorConfigs[executor] = schema || config;
        } else {
          executorConfigs[executor] = config;
        }
      } catch (error) {
        handleInlineError(error, `Error loading config for executor ${executor}`);
      }
    }
  }

  async function loadFlowConfig() {
    loading = true;
    
    try {
      const config = await apiClient.flows.getConfig(namespace, flowId);
      
      // Transform the config data to match our form state
      flow.metadata = {
        id: flowId,
        name: config.metadata.name,
        description: config.metadata.description || '',
        namespace: namespace
      };
      
      // Transform inputs
      flow.inputs = (config.inputs || []).map(input => ({
        ...input,
        optionsText: input.options ? input.options.join('\n') : ''
      }));
      
      // Transform actions
      flow.actions = (config.actions || []).map((action, index) => ({
        tempId: Date.now() + index,
        ...action,
        artifactsText: action.artifacts ? action.artifacts.join('\n') : '',
        on: action.on ? action.on.join(',') : '',
        collapsed: false
      }));
      
      // Load executor configs for all actions
      if (flow.actions.length > 0) {
        await loadExecutorConfigs(flow.actions);
      }
      
    } catch (error: any) {
      handleInlineError(error, 'Error loading flow config');
    } finally {
      loading = false;
    }
  }

  function addInput() {
    flow.inputs.push({
      name: '',
      type: 'string',
      label: '',
      description: '',
      required: false,
      default: '',
      validation: '',
      options: [],
      optionsText: ''
    });
  }

  function addAction() {
    const tempId = Date.now() + Math.random();
    flow.actions.push({
      tempId: tempId,
      id: '',
      name: '',
      executor: '',
      with: {},
      on: '',
      variables: [],
      approval: false,
      artifacts: [],
      artifactsText: '',
      condition: '',
      collapsed: false
    });
  }

  async function updateFlow() {
    saving = true;
    
    try {
      // Transform the flow data to match the API schema for update
      const flowData: FlowUpdateReq = {
        inputs: flow.inputs
          .filter(i => i.name)
          .map((input): FlowInputReq => ({
            name: input.name,
            type: input.type,
            label: input.label || undefined,
            description: input.description || undefined,
            validation: input.validation || undefined,
            required: input.required || false,
            default: input.default || undefined,
            options: input.type === 'select' && input.optionsText 
              ? input.optionsText.split('\n').filter(o => o.trim()) 
              : undefined
          })),
        actions: flow.actions
          .filter(a => a.name && a.id)
          .map((action): FlowActionReq => ({
            id: action.id,
            name: action.name,
            executor: action.executor as 'script' | 'docker',
            with: action.with || {},
            approval: action.approval || false,
            variables: action.variables?.length ? action.variables : undefined,
            artifacts: action.artifactsText 
              ? action.artifactsText.split('\n').filter(a => a.trim())
              : undefined,
            condition: action.condition || undefined,
            on: action.on ? action.on.split(',').map(n => n.trim()).filter(n => n) : undefined
          }))
      };

      await apiClient.flows.update(namespace, flowId, flowData);
      showSuccess('Flow Updated', 'Flow configuration has been updated successfully');
      
      // Redirect to the flow detail page
      await goto(`/view/${namespace}/flows/${flowId}`);
      
    } catch (error: any) {
      handleInlineError(error, 'Error updating flow');
    } finally {
      saving = false;
    }
  }
</script>

<svelte:head>
  <title>Edit Flow - {flow.metadata.name || 'Loading...'} | Flowctl</title>
</svelte:head>

<div class="flex h-screen bg-gray-50">
  <!-- Main Content -->
  <div class="flex-1 flex flex-col overflow-hidden">
    <Header 
      breadcrumbs={[namespace, 'Flows', flow.metadata.name || 'Loading...', 'Edit']}
      actions={[
        { label: 'Cancel', onClick: () => goto(`/view/${namespace}/flows/${flowId}`), variant: 'secondary' }
      ]}
    />

    <!-- Page Content -->
    <div class="flex-1 overflow-y-auto">
      <div class="max-w-6xl mx-auto p-6">
        {#if loading}
          <div class="bg-white rounded-lg shadow-sm border border-gray-200 p-8">
            <div class="animate-pulse">
              <div class="h-4 bg-gray-200 rounded w-1/4 mb-4"></div>
              <div class="h-4 bg-gray-200 rounded w-1/2 mb-2"></div>
              <div class="h-4 bg-gray-200 rounded w-1/3"></div>
            </div>
          </div>
        {:else}
          <FlowMetadata bind:metadata={flow.metadata} readonly={true} />
          <FlowInputs bind:inputs={flow.inputs} {addInput} />
          <FlowActions 
            bind:actions={flow.actions} 
            {addAction} 
            {availableExecutors}
            bind:executorConfigs={executorConfigs}
          />
          
          <!-- Submit Button at Bottom -->
          <div class="bg-white rounded-lg shadow-sm border border-gray-200 p-6 mt-6">
            <div class="flex justify-end space-x-3">
              <button 
                type="button"
                onclick={() => goto(`/view/${namespace}/flows/${flowId}`)}
                class="px-6 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500"
              >
                Cancel
              </button>
              <button 
                type="button"
                onclick={updateFlow}
                disabled={saving}
                class="px-6 py-2 text-sm font-medium text-white bg-blue-600 border border-transparent rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 disabled:opacity-50 disabled:cursor-not-allowed"
              >
                {saving ? 'Updating...' : 'Update Flow'}
              </button>
            </div>
          </div>
        {/if}
      </div>
    </div>
  </div>
</div>

{#if showValidation}
  <ValidationModal 
    bind:show={showValidation}
    {validationResult}
  />
{/if}