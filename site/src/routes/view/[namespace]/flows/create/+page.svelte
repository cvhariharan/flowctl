<script lang="ts">
    import { page } from "$app/stores";
    import { goto } from "$app/navigation";
    import { apiClient } from "$lib/apiClient.js";
    import Header from "$lib/components/shared/Header.svelte";
    import FlowBuilder from "$lib/components/flow-builder/FlowBuilder.svelte";
    import type { BuilderFlow } from "$lib/types.js";
    import {
        emptyFlow,
        newAction,
        toBuilderFlow,
        toFlowRequest,
    } from "$lib/utils/flowBuilder";
    import { handleInlineError, showSuccess } from "$lib/utils/errorHandling";
    import { IconInfoCircle } from "@tabler/icons-svelte";

    let { data } = $props();
    const namespace = $page.params.namespace as string;

    let flow = $state<BuilderFlow>(initialFlow());
    let saving = $state(false);

    function initialFlow(): BuilderFlow {
        if (!data.prefillFlow) {
            const blank = emptyFlow();
            blank.actions.push(newAction());
            return blank;
        }

        const prefilled = toBuilderFlow(data.prefillFlow);
        prefilled.metadata.name = prefilled.metadata.name
            ? `${prefilled.metadata.name} copy`
            : "";
        return prefilled;
    }

    async function saveFlow() {
        saving = true;

        try {
            const result = await apiClient.flows.create(namespace, toFlowRequest(flow));
            showSuccess("Flow Created", `Flow "${flow.metadata.name}" has been created.`);
            await goto(`/view/${encodeURIComponent(namespace)}/flows/${result.id}`);
        } catch (error: any) {
            handleInlineError(error, "Unable to Create Flow");
        } finally {
            saving = false;
        }
    }
</script>

<svelte:head>
    <title>Create Flow - {namespace} | Flowctl</title>
</svelte:head>

<Header
    breadcrumbs={[
        { label: namespace, url: `/view/${encodeURIComponent(namespace)}/flows` },
        { label: "Flows", url: `/view/${encodeURIComponent(namespace)}/flows` },
        { label: "Create" },
    ]}
/>

{#if data.prefillFlow}
    <div role="alert" class="hstack gap-2 items-start duplicate-note">
        <IconInfoCircle class="shrink-0" size={16} />
        <span>
            Secrets are not copied. Re-add them under <strong>Secrets</strong> once this flow exists.
        </span>
    </div>
{/if}

<FlowBuilder
    bind:flow
    {namespace}
    mode="create"
    {saving}
    availableExecutors={data.availableExecutors}
    availableMessengers={data.availableMessengers || []}
    messengerConfigs={data.messengerConfigs || {}}
    onSave={saveFlow}
    onCancel={() => goto(`/view/${encodeURIComponent(namespace)}/flows`)}
/>

<style>
    .duplicate-note {
        margin: var(--space-4) var(--space-6) 0;
    }
</style>
