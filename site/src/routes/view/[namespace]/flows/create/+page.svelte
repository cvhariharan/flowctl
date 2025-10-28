<script lang="ts">
    import { onMount } from "svelte";
    import { page } from "$app/stores";
    import { apiClient } from "$lib/apiClient.js";
    import Header from "$lib/components/shared/Header.svelte";
    import FlowMetadata from "$lib/components/flow-create/FlowMetadata.svelte";
    import FlowInputs from "$lib/components/flow-create/FlowInputs.svelte";
    import FlowActions from "$lib/components/flow-create/FlowActions.svelte";
    import ValidationModal from "$lib/components/flow-create/ValidationModal.svelte";
    import Tabs from "$lib/components/shared/Tabs.svelte";
    import type { PageData } from "./$types";
    import type {
        FlowCreateReq,
        FlowInputReq,
        FlowActionReq,
    } from "$lib/types.js";
    import { goto } from "$app/navigation";
    import { handleInlineError, showSuccess } from "$lib/utils/errorHandling";

    let { data }: { data: PageData } = $props();
    const namespace = $page.params.namespace as string;

    // Flow state
    let flow = $state({
        metadata: {
            id: "",
            name: "",
            description: "",
            schedules: [] as string[],
            namespace: namespace,
        },
        inputs: [] as any[],
        actions: [] as any[],
    });

    // Modal states
    let showValidation = $state(false);
    let validationResult = $state({
        success: false,
        errors: [] as string[],
    });

    // Loading states
    let saving = $state(false);
    const availableExecutors = data.availableExecutors;

    // Executor configs for actions
    let executorConfigs = $state({} as Record<string, any>);

    // Tab state
    let activeTab = $state("metadata");

    const tabs = [
        { id: "metadata", label: "General" },
        { id: "inputs", label: "Inputs" },
        { id: "actions", label: "Actions" },
    ];

    onMount(() => {
        // Add first action by default
        addAction();
    });

    function addInput() {
        flow.inputs.push({
            name: "",
            type: "string",
            label: "",
            description: "",
            required: false,
            default: "",
            validation: "",
            options: [],
            optionsText: "",
        });
    }

    function addAction() {
        const tempId = Date.now() + Math.random();
        flow.actions.push({
            tempId: tempId,
            id: "",
            name: "",
            executor: "",
            with: {},
            selectedNodes: [],
            variables: [],
            approval: false,
            artifacts: [],
            condition: "",
            collapsed: false,
        });
    }

    async function saveFlow() {
        saving = true;

        try {
            // Transform the flow data to match the API schema
            const flowData: FlowCreateReq = {
                metadata: {
                    name: flow.metadata.name,
                    description: flow.metadata.description || undefined,
                    schedules:
                        flow.metadata.schedules?.filter((s) => s.trim()) ||
                        undefined,
                },
                inputs: flow.inputs
                    .filter((i) => i.name)
                    .map(
                        (input): FlowInputReq => ({
                            name: input.name,
                            type: input.type,
                            label: input.label || undefined,
                            description: input.description || undefined,
                            validation: input.validation || undefined,
                            required: input.required || false,
                            default: input.default || undefined,
                            options:
                                input.type === "select" && input.optionsText
                                    ? input.optionsText
                                          .split("\n")
                                          .filter((o: string) => o.trim())
                                    : undefined,
                        }),
                    ),
                actions: flow.actions
                    .filter((a) => a.name && a.id)
                    .map(
                        (action): FlowActionReq => ({
                            name: action.name,
                            executor: action.executor as "script" | "docker",
                            with: action.with || {},
                            approval: action.approval || false,
                            variables: action.variables
                                ?.filter((v: any) => v.name && v.name.trim())
                                .map((v: any) => ({ [v.name]: v.value })),
                            artifacts:
                                action.artifacts && action.artifacts.length > 0
                                    ? action.artifacts.filter((a: string) =>
                                          a.trim(),
                                      )
                                    : undefined,
                            condition: action.condition || undefined,
                            on: action.selectedNodes?.length
                                ? action.selectedNodes
                                : undefined,
                        }),
                    ),
            };

            const result = await apiClient.flows.create(namespace, flowData);

            showSuccess(
                "Flow Created",
                `Flow "${flow.metadata.name}" has been created successfully.`,
            );

            // Redirect to flow detail page
            await goto(`/view/${namespace}/flows/${result.id}`);
        } catch (error: any) {
            handleInlineError(error, "Unable to Create Flow");
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
            breadcrumbs={[
                { label: namespace, url: `/view/${namespace}/flows` },
                { label: "Flows", url: `/view/${namespace}/flows` },
                { label: "Create" },
            ]}
        />

        <!-- Page Content -->
        <div class="flex-1 overflow-y-auto bg-gray-50">
            <div class="max-w-6xl mx-auto px-6 py-8">
                <!-- Page Title -->
                <div class="mb-8">
                    <h1 class="text-2xl font-bold text-gray-900">
                        Create Flow
                    </h1>
                    <p class="mt-1 text-sm text-gray-600">
                        Define a new workflow
                    </p>
                </div>

                <!-- Main Card -->
                <div class="bg-white rounded-lg border border-gray-300">
                    <!-- Tab Navigation -->
                    <div class="border-b border-gray-200">
                        <Tabs bind:activeTab {tabs} />
                    </div>

                    <!-- Tab Content -->
                    <div class="p-6">
                        {#if activeTab === "metadata"}
                            <FlowMetadata
                                bind:metadata={flow.metadata}
                                inputs={flow.inputs}
                            />
                        {:else if activeTab === "inputs"}
                            <FlowInputs bind:inputs={flow.inputs} {addInput} />
                        {:else if activeTab === "actions"}
                            <FlowActions
                                {namespace}
                                bind:actions={flow.actions}
                                {addAction}
                                {availableExecutors}
                                bind:executorConfigs
                            />
                        {/if}
                    </div>

                    <!-- Action Buttons -->
                    <div
                        class="px-6 py-4 bg-gray-50 border-t border-gray-200 flex justify-end gap-3"
                    >
                        <button
                            type="button"
                            onclick={() => goto(`/view/${namespace}/flows`)}
                            class="px-6 py-2 text-sm font-medium cursor-pointer text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-primary-400"
                        >
                            Cancel
                        </button>
                        <button
                            type="button"
                            onclick={saveFlow}
                            disabled={saving}
                            class="px-6 py-2 text-sm font-medium cursor-pointer text-white bg-primary-500 border border-transparent rounded-md hover:bg-primary-600 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-primary-400 disabled:opacity-50 disabled:cursor-not-allowed"
                        >
                            {saving ? "Creating..." : "Create Flow"}
                        </button>
                    </div>
                </div>
            </div>
        </div>
    </div>
</div>

{#if showValidation}
    <ValidationModal bind:show={showValidation} {validationResult} />
{/if}
