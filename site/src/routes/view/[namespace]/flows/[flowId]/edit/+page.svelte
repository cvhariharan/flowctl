<script lang="ts">
    import { onMount } from "svelte";
    import { page } from "$app/stores";
    import { goto } from "$app/navigation";
    import { apiClient } from "$lib/apiClient.js";
    import Header from "$lib/components/shared/Header.svelte";
    import FlowBuilder from "$lib/components/flow-builder/FlowBuilder.svelte";
    import type { BuilderFlow, FlowUpdateReq } from "$lib/types.js";
    import { emptyFlow, toBuilderFlow, toFlowRequest } from "$lib/utils/flowBuilder";
    import { handleInlineError, showSuccess } from "$lib/utils/errorHandling";

    let { data } = $props();
    const namespace = $page.params.namespace as string;
    const flowId = $page.params.flowId as string;
    const readonly = $derived(data.readonly ?? false);

    let flow = $state<BuilderFlow>(emptyFlow(flowId));
    let loading = $state(true);
    let saving = $state(false);

    onMount(loadFlowConfig);

    async function loadFlowConfig() {
        loading = true;

        try {
            const config = await apiClient.flows.getConfig(namespace, flowId);
            flow = toBuilderFlow(config, flowId);
        } catch (error: any) {
            handleInlineError(error, "Error loading flow config");
        } finally {
            loading = false;
        }
    }

    async function updateFlow() {
        saving = true;

        try {
            const { metadata, ...body } = toFlowRequest(flow);
            const { name, ...meta } = metadata;
            const flowData: FlowUpdateReq = { ...meta, ...body, schedules: meta.schedules ?? [] };

            await apiClient.flows.update(namespace, flowId, flowData);
            showSuccess("Flow Updated", "Flow configuration has been updated.");
            await goto(`/view/${encodeURIComponent(namespace)}/flows/${flowId}`);
        } catch (error: any) {
            handleInlineError(error, "Error updating flow");
        } finally {
            saving = false;
        }
    }
</script>

<svelte:head>
    <title>
        {readonly ? "View" : "Edit"} Flow - {flow.metadata.name || "Loading..."} | Flowctl
    </title>
</svelte:head>

<Header
    breadcrumbs={[
        { label: namespace, url: `/view/${encodeURIComponent(namespace)}/flows` },
        { label: "Flows", url: `/view/${encodeURIComponent(namespace)}/flows` },
        ...(flow.metadata.prefix
            ? [
                  {
                      label: flow.metadata.prefix,
                      url: `/view/${encodeURIComponent(namespace)}/flows?group=${encodeURIComponent(flow.metadata.prefix)}`,
                  },
              ]
            : []),
        {
            label: flow.metadata.name || "Loading...",
            url: `/view/${encodeURIComponent(namespace)}/flows/${flowId}`,
        },
        { label: readonly ? "View Config" : "Edit" },
    ]}
/>

{#if loading}
    <div class="page-content">
        <div class="card p-4" aria-busy="true">
            <div class="skeleton line mb-4" style="width: 25%" role="status"></div>
            <div class="skeleton line mb-2" style="width: 50%" role="status"></div>
            <div class="skeleton line" style="width: 25%" role="status"></div>
        </div>
    </div>
{:else}
    <FlowBuilder
        bind:flow
        {namespace}
        {flowId}
        mode="edit"
        {saving}
        {readonly}
        availableExecutors={data.availableExecutors}
        availableMessengers={data.availableMessengers || []}
        messengerConfigs={data.messengerConfigs || {}}
        onSave={updateFlow}
        onCancel={() => goto(`/view/${encodeURIComponent(namespace)}/flows/${flowId}`)}
    />
{/if}
