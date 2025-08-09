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
  import type { FlowCreateReq, FlowInputReq, FlowActionReq } from '$lib/types.js';
  import { goto } from '$app/navigation';

  let { data }: { data: PageData } = $props();
  const namespace = $page.params.namespace as string;

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

  // Loading and error states
  let saving = $state(false);
  let saveError = $state('');
  const availableExecutors = data.availableExecutors;
  
  // Executor configs for actions
  let executorConfigs = $state({} as Record<string, any>);

  onMount(() => {
    // Add first action by default
    addAction();
  });

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

  async function saveFlow() { 
    saving = true;
    saveError = '';
    
    try {
      // Transform the flow data to match the API schema
      const flowData: FlowCreateReq = {
        metadata: {
          name: flow.metadata.name,
          description: flow.metadata.description || undefined
        },
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

      const result = await apiClient.flows.create(namespace, flowData);
      console.log('Flow created successfully:', result);
      
      // Redirect to the flows list or flow detail page
      await goto(`/view/${namespace}/flows`);
      
    } catch (error: any) {
      console.error('Error saving flow:', error);
      if (error.data?.error) {
        saveError = error.data.error;
      } else {
        saveError = 'Failed to create flow. Please try again.';
      }
    } finally {
      saving = false;
    }
  }

  // Remove header actions - buttons moved to bottom
</script>

<svelte:head>
  <title>Create Flow - {namespace} | Flowctl</title>
</svelte:head>

<div class="flex h-screen bg-gray-50">
  <!-- Main Content -->
  <div class="flex-1 flex flex-col overflow-hidden">
    <Header 
      breadcrumbs={[namespace, 'Flows', 'Create']}
    />

    <!-- Page Content -->
    <div class="flex-1 overflow-y-auto">
      <div class="max-w-6xl mx-auto p-6">
        <FlowMetadata bind:metadata={flow.metadata} />
        <FlowInputs bind:inputs={flow.inputs} {addInput} />
        <FlowActions 
          bind:actions={flow.actions} 
          {addAction} 
          {availableExecutors}
          bind:executorConfigs={executorConfigs}
        />
        
        <!-- Submit Button at Bottom -->
        <div class="bg-white rounded-lg shadow-sm border border-gray-200 p-6 mt-6">
          {#if saveError}
            <div class="mb-4 p-4 bg-red-50 border border-red-200 rounded-md">
              <div class="flex">
                <div class="flex-shrink-0">
                  <svg class="h-5 w-5 text-red-400" viewBox="0 0 20 20" fill="currentColor">
                    <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd" />
                  </svg>
                </div>
                <div class="ml-3">
                  <p class="text-sm text-red-800">{saveError}</p>
                </div>
              </div>
            </div>
          {/if}
          
          <div class="flex justify-end">
            <button 
              type="button"
              onclick={saveFlow}
              disabled={saving}
              class="px-6 py-2 text-sm font-medium text-white bg-blue-600 border border-transparent rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {saving ? 'Creating...' : 'Create Flow'}
            </button>
          </div>
        </div>
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