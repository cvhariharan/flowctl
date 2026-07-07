<script lang="ts">
    import { onMount } from "svelte";
    import { page } from "$app/stores";
    import { apiClient } from "$lib/apiClient.js";
    import Header from "$lib/components/shared/Header.svelte";
    import PageHeader from "$lib/components/shared/PageHeader.svelte";
    import FlowMetadata from "$lib/components/flow-create/FlowMetadata.svelte";
    import FlowInputs from "$lib/components/flow-create/FlowInputs.svelte";
    import FlowActions from "$lib/components/flow-create/FlowActions.svelte";
    import FlowNotifications from "$lib/components/flow-create/FlowNotifications.svelte";
    import ValidationModal from "$lib/components/flow-create/ValidationModal.svelte";
    import SecretsTab from "$lib/components/secrets/SecretsTab.svelte";
    import type { PageData } from "./$types";
    import type {
        FlowUpdateReq,
        FlowInputReq,
        FlowActionReq,
        Schedule,
    } from "$lib/types.js";
    import { goto } from "$app/navigation";
    import { handleInlineError, showSuccess } from "$lib/utils/errorHandling";

    let { data }: { data: PageData } = $props();
    const namespace = $page.params.namespace as string;
    const flowId = $page.params.flowId as string;
    const readonly = data.readonly ?? false;

    // Flow state
    let flow = $state({
        metadata: {
            id: "",
            name: "",
            description: "",
            prefix: "",
            schedules: [] as Schedule[],
            namespace: namespace,
            allow_overlap: false,
            user_schedulable: false,
            max_retries: 0,
        },
        inputs: [] as any[],
        actions: [] as any[],
        notifications: [] as any[],
    });

    // Modal states
    let showValidation = $state(false);
    let validationResult = $state({
        success: false,
        errors: [] as string[],
    });

    // Loading states
    let loading = $state(true);
    let saving = $state(false);
    const availableExecutors = data.availableExecutors;
    const availableMessengers = data.availableMessengers || [];

    // Executor configs for actions
    let executorConfigs = $state({} as Record<string, any>);

    // Messenger configs for notifications (pre-loaded in page loader)
    const messengerConfigs = data.messengerConfigs || {};

    let formElement: HTMLFormElement;
    let onSecretsTab = $state(false);

    onMount(async () => {
        await loadFlowConfig();
    });

    async function loadExecutorConfigs(actions: any[]) {
        const executorTypes = [
            ...new Set(
                actions.map((action) => action.executor).filter(Boolean),
            ),
        ];

        for (const executor of executorTypes) {
            try {
                const config = await apiClient.executors.getConfig(executor);

                // Handle both direct schema and $ref-based schemas
                if (config.$defs && config.$ref) {
                    const refPath = config.$ref.replace("#/$defs/", "");
                    const schema = config.$defs[refPath];
                    executorConfigs[executor] = schema || config;
                } else {
                    executorConfigs[executor] = config;
                }
            } catch (error) {
                handleInlineError(
                    error,
                    `Error loading config for executor ${executor}`,
                );
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
                description: config.metadata.description || "",
                prefix: config.metadata.prefix || "",
                schedules: config.metadata.schedules || [],
                namespace: namespace,
                allow_overlap: config.metadata.allow_overlap || false,
                user_schedulable: config.metadata.user_schedulable || false,
                max_retries: config.metadata.max_retries || 0,
            };

            // Transform inputs
            flow.inputs = (config.inputs || []).map((input) => ({
                ...input,
                optionsText: input.options ? input.options.join("\n") : "",
                maxFileSizeMB: input.max_file_size
                    ? input.max_file_size / 1024 / 1024
                    : undefined,
                useRemoteOptions: !!input.remote_options,
                remoteHeaders: input.remote_options?.headers
                    ? Object.entries(input.remote_options.headers).map(([key, value]) => ({ key, value }))
                    : [],
            }));

            // Transform actions
            flow.actions = (config.actions || []).map((action, index) => ({
                tempId: Date.now() + index,
                ...action,
                // Transform variables from API format {key: value} to UI format {name: key, value: value}
                variables: action.variables
                    ? action.variables.map((varObj) => {
                          const [key, value] = Object.entries(varObj)[0];
                          return { name: key, value: value };
                      })
                    : [],
                artifacts: action.artifacts || [],
                selectedNodes: action.on || [],
                collapsed: false,
            }));

            // Transform notifications
            if (config.notify && Array.isArray(config.notify)) {
                flow.notifications = config.notify.map((notification) => ({
                    channel: notification.channel || "email",
                    events: notification.events || [],
                    config: notification.config || {},
                }));
            } else {
                flow.notifications = [];
            }

            // Load executor configs for all actions
            if (flow.actions.length > 0) {
                await loadExecutorConfigs(flow.actions);
            }

        } catch (error: any) {
            handleInlineError(error, "Error loading flow config");
        } finally {
            loading = false;
        }
    }

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

    function addNotification() {
        flow.notifications.push({
            channel: "",
            events: [],
            config: {},
        });
    }

    function validateFlow(): boolean {
        const errors: string[] = [];

        if (!flow.metadata.name?.trim()) {
            errors.push("Flow name is required.");
        }

        for (const [i, action] of flow.actions.entries()) {
            const label = `Action ${i + 1}`;
            if (!action.name?.trim()) {
                errors.push(`${label}: Action name is required.`);
            }
            if (!action.executor) {
                errors.push(`${label} ("${action.name || "Untitled"}"): Executor is required.`);
            }
        }

        for (const [i, input] of flow.inputs.entries()) {
            if (!input.name?.trim()) {
                errors.push(`Input ${i + 1}: Input name is required.`);
            }
        }

        if (errors.length > 0) {
            validationResult = { success: false, errors };
            showValidation = true;
            return false;
        }

        return true;
    }

    async function updateFlow() {
        saving = true;

        try {
            // Transform the flow data to match the API schema for update
            const flowData: FlowUpdateReq = {
                prefix: flow.metadata.prefix || undefined,
                schedules:
                    flow.metadata.schedules?.filter((s) => s.cron.trim()) || [],
                allow_overlap: flow.metadata.allow_overlap,
                user_schedulable: flow.metadata.user_schedulable,
                max_retries: flow.metadata.max_retries || 0,
                description: flow.metadata.description || undefined,
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
                                input.type === "select" && !input.useRemoteOptions && input.optionsText
                                    ? input.optionsText
                                          .split("\n")
                                          .filter((o: string) => o.trim())
                                    : undefined,
                            remote_options:
                                input.type === "select" && input.useRemoteOptions && input.remote_options?.url
                                    ? {
                                          url: input.remote_options.url,
                                          method: input.remote_options.method || undefined,
                                          headers: Object.keys(input.remote_options.headers ?? {}).length > 0
                                              ? input.remote_options.headers
                                              : undefined,
                                      }
                                    : undefined,
                            max_file_size: input.max_file_size || undefined,
                        }),
                    ),
                actions: flow.actions
                    .filter((a) => a.name)
                    .map(
                        (action): FlowActionReq => ({
                            name: action.name,
                            executor: action.executor as "script" | "docker",
                            with: action.with || {},
                            approval: action.approval || false,
                            approvers: action.approvers?.length ? action.approvers : undefined,
                            approval_groups: action.approval_groups?.length ? action.approval_groups : undefined,
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
                notify: flow.notifications
                    .filter((n) => n.channel)
                    .map((notification) => ({
                        channel: notification.channel,
                        events: notification.events || [],
                        config: notification.config || {},
                    })),
            };

            await apiClient.flows.update(namespace, flowId, flowData);
            showSuccess(
                "Flow Updated",
                "Flow configuration has been updated successfully",
            );

            // Redirect to the flow detail page
            await goto(`/view/${encodeURIComponent(namespace)}/flows/${flowId}`);
        } catch (error: any) {
            handleInlineError(error, "Error updating flow");
        } finally {
            saving = false;
        }
    }
</script>

<svelte:head>
    <title>{readonly ? "View" : "Edit"} Flow - {flow.metadata.name || "Loading..."} | Flowctl</title>
</svelte:head>

<Header
    breadcrumbs={[
        { label: namespace, url: `/view/${encodeURIComponent(namespace)}/flows` },
        { label: "Flows", url: `/view/${encodeURIComponent(namespace)}/flows` },
        ...(flow.metadata.prefix
            ? [{ label: flow.metadata.prefix, url: `/view/${encodeURIComponent(namespace)}/flows?group=${encodeURIComponent(flow.metadata.prefix)}` }]
            : []),
        {
            label: flow.metadata.name || "Loading...",
            url: `/view/${encodeURIComponent(namespace)}/flows/${flowId}`,
        },
        { label: readonly ? "View Config" : "Edit" },
    ]}
/>

<div class="page-content">
        {#if loading}
            <div class="card p-4" aria-busy="true">
                <div class="skeleton line mb-4" style="width: 25%" role="status"></div>
                <div class="skeleton line mb-2" style="width: 50%" role="status"></div>
                <div class="skeleton line" style="width: 25%" role="status"></div>
            </div>
        {:else}
            <PageHeader
                title={readonly ? "View Flow Config" : "Edit Flow"}
                subtitle={readonly
                    ? `Viewing read-only configuration for ${flow.metadata.name}`
                    : `Update workflow configuration for ${flow.metadata.name}`}
            />

                    <form bind:this={formElement}>
                        <ot-tabs onot-tab-change={(e: CustomEvent) => { onSecretsTab = !readonly && e.detail.index === 4; }}>
                            <div role="tablist">
                                <button role="tab" aria-selected="true">General</button>
                                <button role="tab">Inputs</button>
                                <button role="tab">Actions</button>
                                <button role="tab">Notifications</button>
                                {#if !readonly}
                                    <button role="tab">Secrets</button>
                                {/if}
                            </div>
                            <div role="tabpanel">
                                <fieldset disabled={readonly} class="contents">
                                    <FlowMetadata
                                        bind:metadata={flow.metadata}
                                        {namespace}
                                        inputs={flow.inputs}
                                        updatemode={true}
                                        disabled={readonly}
                                    />
                                </fieldset>
                            </div>
                            <div role="tabpanel">
                                <fieldset disabled={readonly} class="contents">
                                    <FlowInputs
                                        bind:inputs={flow.inputs}
                                        {addInput}
                                        disabled={readonly}
                                    />
                                </fieldset>
                            </div>
                            <div role="tabpanel">
                                <fieldset disabled={readonly} class="contents">
                                    <FlowActions
                                        {namespace}
                                        bind:actions={flow.actions}
                                        {addAction}
                                        {availableExecutors}
                                        bind:executorConfigs
                                        disabled={readonly}
                                    />
                                </fieldset>
                            </div>
                            <div role="tabpanel">
                                <fieldset disabled={readonly} class="contents">
                                    <FlowNotifications
                                        bind:notifications={flow.notifications}
                                        {addNotification}
                                        {availableMessengers}
                                        {messengerConfigs}
                                        disabled={readonly}
                                    />
                                </fieldset>
                            </div>
                            {#if !readonly}
                                <div role="tabpanel">
                                    <SecretsTab {namespace} {flowId} />
                                </div>
                            {/if}
                        </ot-tabs>

                        <!-- Action Buttons -->
                        {#if !onSecretsTab}
                            <div class="hstack gap-2 justify-end mt-6">
                                <button
                                    type="button"
                                    onclick={() =>
                                        goto(
                                            `/view/${encodeURIComponent(namespace)}/flows/${flowId}`,
                                        )}
                                    data-variant="secondary"
                                >
                                    {readonly ? "Back" : "Cancel"}
                                </button>
                                {#if !readonly}
                                    <button
                                        type="button"
                                        onclick={() => {
                                            if (validateFlow()) {
                                                updateFlow();
                                            }
                                        }}
                                        disabled={saving}
                                    >
                                        <span class="hstack gap-2 justify-center" aria-busy={saving} data-spinner="small">
                                            {saving ? "Updating..." : "Update"}
                                        </span>
                                    </button>
                                {/if}
                            </div>
                        {/if}
                    </form>
        {/if}
</div>

{#if showValidation}
    <ValidationModal bind:show={showValidation} {validationResult} />
{/if}

<style>
    .contents {
        display: contents;
    }

    form {
        max-width: 64rem;
    }

    :global([role="tabpanel"]) {
        border: 1px solid var(--border);
        border-radius: var(--radius-medium);
        padding: var(--space-6);
        background: var(--card);
    }
</style>
