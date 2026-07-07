<script lang="ts">
    import { apiClient } from "$lib/apiClient.js";
    import { handleInlineError } from "$lib/utils/errorHandling";
    import { createSlug } from "$lib/utils";
    import { notifications } from "$lib/stores/notifications";
    import OatSelect from "$lib/components/shared/OatSelect.svelte";
    import CodeEditor from "$lib/components/shared/CodeEditor.svelte";
    import KeyValueEditor from "$lib/components/shared/KeyValueEditor.svelte";
    import NodeSelector from "$lib/components/shared/NodeSelector.svelte";
    import type { NodeResp } from "$lib/types";

    let {
        namespace,
        actions = $bindable(),
        addAction,
        availableExecutors,
        executorConfigs = $bindable(),
        disabled = false,
    }: {
        namespace: string;
        actions: any[];
        addAction: () => void;
        availableExecutors: Array<{ name: string; capabilities: string[] }>;
        executorConfigs: Record<string, any>;
        disabled?: boolean;
    } = $props();
    let draggedIndex: number | null = null;

    function executorHasCapability(executorName: string, capability: string): boolean {
        const exec = availableExecutors.find(e => e.name === executorName);
        return exec?.capabilities?.includes(capability) ?? false;
    }

    function handleAddAction() {
        addAction();
        notifications.info(
            "Action Added",
            "New action has been added to the flow",
        );
    }

    function removeAction(index: number) {
        actions.splice(index, 1);
    }

    function duplicateAction(index: number) {
        const original = actions[index];
        const tempId = Date.now() + Math.random();
        const duplicate = JSON.parse(JSON.stringify(original));

        duplicate.tempId = tempId;
        duplicate.id = original.id ? original.id + "_copy" : "";
        duplicate.name = original.name ? original.name + " (Copy)" : "";

        actions.splice(index + 1, 0, duplicate);
    }

    function updateActionName(action: any, value: string) {
        action.name = value;
        // Auto-generate ID from name
        action.id = createSlug(value);
    }

    async function onExecutorChange(action: any) {
        if (!action.executor) {
            action.with = {};
            return;
        }

        try {
            const config = await apiClient.executors.getConfig(action.executor);
            executorConfigs[action.executor] = config;
            action.with = {};

            // Initialize with default values
            if (config.$defs && config.$ref) {
                const refPath = config.$ref.replace("#/$defs/", "");
                const schema = config.$defs[refPath];
                if (schema && schema.properties) {
                    Object.entries(schema.properties).forEach(
                        ([key, property]: [string, any]) => {
                            if (property.default !== undefined) {
                                action.with[key] = property.default;
                            }
                        },
                    );
                }
                executorConfigs[action.executor] = schema || config;
            } else if (config.properties) {
                Object.entries(config.properties).forEach(
                    ([key, property]: [string, any]) => {
                        if (property.default !== undefined) {
                            action.with[key] = property.default;
                        }
                    },
                );
            }
        } catch (error) {
            handleInlineError(error, "Unable to Load Executor Configuration");
        }
    }

    function updateConfigValue(action: any, key: string, value: any) {
        if (!action.with) {
            action.with = {};
        }
        action.with[key] = value;
    }

    // Drag and drop functions
    function dragStart(event: DragEvent, index: number) {
        draggedIndex = index;
        if (event.target instanceof HTMLElement) {
            event.target.classList.add("opacity-50");
        }
    }

    function dragEnd(event: DragEvent) {
        if (event.target instanceof HTMLElement) {
            event.target.classList.remove("opacity-50");
        }
        draggedIndex = null;
    }

    function dragOver(event: DragEvent) {
        event.preventDefault();
        if (event.currentTarget instanceof HTMLElement) {
            event.currentTarget.classList.add("drag-over");
        }
    }

    function dragLeave(event: DragEvent) {
        if (event.currentTarget instanceof HTMLElement) {
            event.currentTarget.classList.remove("drag-over");
        }
    }

    function drop(event: DragEvent, dropIndex: number) {
        event.preventDefault();
        if (event.currentTarget instanceof HTMLElement) {
            event.currentTarget.classList.remove("drag-over");
        }
        if (draggedIndex !== null && draggedIndex !== dropIndex) {
            const dragged = actions.splice(draggedIndex, 1)[0];
            actions.splice(dropIndex, 0, dragged);
        }
    }
</script>

<!-- Flow Actions Section -->
<div>
    <div class="hstack mb-4 justify-between">
        <h3>Flow Actions</h3>
        {#if !disabled}
            <button onclick={handleAddAction}>
                + Add Action
            </button>
        {/if}
    </div>

    <div class="vstack gap-4">
        {#each actions as action, index (action.tempId)}
            <div
                class="action-card"
                ondragover={dragOver}
                ondragleave={dragLeave}
                ondrop={(e) => drop(e, index)}
            >
                <!-- Action Header -->
                <div
                    class="action-header hstack"
                    draggable="true"
                    ondragstart={(e) => dragStart(e, index)}
                    ondragend={dragEnd}
                >
                    <div class="hstack gap-2">
                        <svg class="drag-icon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16" />
                        </svg>
                        <span class="action-name">{action.name || "Untitled Action"}</span>
                        <span class="action-badge">{action.executor || "No executor"}</span>
                    </div>
                    <div class="hstack gap-1">
                        <button
                            type="button"
                            data-variant="secondary"
                            class="ghost icon small"
                            onclick={() => (action.collapsed = !action.collapsed)}
                        >
                            <svg class="icon-sm" class:rotated={!action.collapsed} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
                            </svg>
                        </button>
                        {#if !disabled}
                            <button
                                type="button"
                                data-variant="secondary"
                                class="ghost icon small"
                                onclick={() => duplicateAction(index)}
                            >
                                <svg class="icon-sm" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
                                </svg>
                            </button>
                            <button
                                type="button"
                                data-variant="danger"
                                class="ghost icon small"
                                onclick={() => removeAction(index)}
                            >
                                <svg class="icon-sm" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                                </svg>
                            </button>
                        {/if}
                    </div>
                </div>

                <!-- Action Content -->
                {#if !action.collapsed}
                    <div class="action-body vstack gap-4">
                        <!-- Basic Action Fields -->
                        <div data-field>
                            <label>Action Name *</label>
                            <input
                                type="text"
                                value={action.name}
                                oninput={(e) => updateActionName(action, e.currentTarget.value)}
                                placeholder="Action Display Name"
                            />
                        </div>

                        <div class="grid-2">
                            <div data-field>
                                <label>Executor *</label>
                                <OatSelect
                                    bind:value={action.executor}
                                    options={availableExecutors.map((e: any) => ({ value: e.name, label: e.name }))}
                                    placeholder="Select Executor"
                                    onchange={() => onExecutorChange(action)}
                                />
                            </div>
                            {#if executorHasCapability(action.executor, 'remote_execution')}
                            <div data-field>
                                <label>Run On</label>
                                <NodeSelector
                                    {namespace}
                                    bind:selectedNodes={action.selectedNodes}
                                    placeholder="Search nodes..."
                                    {disabled}
                                />
                            </div>
                            {/if}
                        </div>

                        <!-- Dynamic Executor Configuration -->
                        {#if action.executor && executorConfigs[action.executor]}
                            <div class="vstack gap-4">
                                <div class="config-section">
                                    <h4 class="hstack gap-2 config-title">
                                        <svg class="icon-sm text-lighter" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
                                        </svg>
                                        <span>{action.executor.charAt(0).toUpperCase() + action.executor.slice(1)}</span>
                                        Configuration
                                    </h4>

                                    <!-- Dynamic form fields based on JSON schema -->
                                    {#if executorConfigs[action.executor].properties}
                                        <div class="vstack gap-4">
                                            {#each Object.entries(executorConfigs[action.executor].properties) as [key, property]}
                                                {@const isRequired = executorConfigs[action.executor].required?.includes(key)}
                                                {@const label = property.title || key}
                                                {@const description = property.description || ""}
                                                {@const placeholder = property.placeholder || property.default || ""}

                                                {#if property.type === "checkbox"}
                                                    <div class="hstack gap-2 items-start">
                                                        <input
                                                            type="checkbox"
                                                            id="config-{action.tempId}-{key}"
                                                            bind:checked={action.with[key]}
                                                            onchange={(e) => updateConfigValue(action, key, e.currentTarget.checked)}
                                                        />
                                                        <div>
                                                            <label for="config-{action.tempId}-{key}">
                                                                {label}
                                                                {#if isRequired}<span class="required">*</span>{/if}
                                                            </label>
                                                            {#if description}
                                                                <p class="text-lighter field-hint">{description}</p>
                                                            {/if}
                                                        </div>
                                                    </div>
                                                {:else if property.enum}
                                                    <div data-field>
                                                        <label for="config-{action.tempId}-{key}">
                                                            {label}
                                                            {#if isRequired}<span class="required">*</span>{/if}
                                                        </label>
                                                        <OatSelect
                                                            bind:value={action.with[key]}
                                                            options={property.enum.map((o: string) => ({ value: o, label: o }))}
                                                            placeholder="Select..."
                                                            id="config-{action.tempId}-{key}"
                                                            onchange={() => updateConfigValue(action, key, action.with[key])}
                                                        />
                                                        {#if description}
                                                            <p class="text-lighter field-hint">{description}</p>
                                                        {/if}
                                                    </div>
                                                {:else if property.type === "number" || property.type === "integer"}
                                                    <div data-field>
                                                        <label for="config-{action.tempId}-{key}">
                                                            {label}
                                                            {#if isRequired}<span class="required">*</span>{/if}
                                                        </label>
                                                        <input
                                                            type="number"
                                                            id="config-{action.tempId}-{key}"
                                                            bind:value={action.with[key]}
                                                            oninput={(e) => updateConfigValue(action, key, property.type === "integer" ? parseInt(e.currentTarget.value) : parseFloat(e.currentTarget.value))}
                                                            step={property.type === "integer" ? "1" : "any"}
                                                            min={property.minimum}
                                                            max={property.maximum}
                                                            {placeholder}
                                                        />
                                                        {#if description}
                                                            <p class="text-lighter field-hint">{description}</p>
                                                        {/if}
                                                    </div>
                                                {:else if property.widget === "codeeditor"}
                                                    <div data-field>
                                                        <label>
                                                            {label}
                                                            {#if isRequired}<span class="required">*</span>{/if}
                                                        </label>
                                                        <CodeEditor
                                                            value={action.with[key] || ""}
                                                            height="200px"
                                                            onchange={(val) => updateConfigValue(action, key, val)}
                                                        />
                                                        {#if description}
                                                            <p class="text-lighter field-hint">{description}</p>
                                                        {/if}
                                                    </div>
                                                {:else if property.widget === "keyvalue"}
                                                    <div data-field>
                                                        <label>
                                                            {label}
                                                            {#if isRequired}<span class="required">*</span>{/if}
                                                        </label>
                                                        <KeyValueEditor
                                                            initialValue={action.with[key]}
                                                            onchange={(json) => updateConfigValue(action, key, json)}
                                                            valuePlaceholder={placeholder || "value"}
                                                        />
                                                        {#if description}
                                                            <p class="text-lighter field-hint">{description}</p>
                                                        {/if}
                                                    </div>
                                                {:else if property.format === "textarea" || property.type === "object" || property.type === "array"}
                                                    <div data-field>
                                                        <label for="config-{action.tempId}-{key}">
                                                            {label}
                                                            {#if isRequired}<span class="required">*</span>{/if}
                                                        </label>
                                                        <textarea
                                                            id="config-{action.tempId}-{key}"
                                                            bind:value={action.with[key]}
                                                            oninput={(e) => updateConfigValue(action, key, e.currentTarget.value)}
                                                            placeholder={placeholder || (property.type === "object" ? "JSON object" : property.type === "array" ? "Array values" : "Multi-line text")}
                                                            rows="4"
                                                        ></textarea>
                                                        {#if description}
                                                            <p class="text-lighter field-hint">{description}</p>
                                                        {/if}
                                                        {#if property.type === "object" || property.type === "array"}
                                                            <p class="text-lighter field-hint">Enter as JSON format</p>
                                                        {/if}
                                                    </div>
                                                {:else}
                                                    <div data-field>
                                                        <label for="config-{action.tempId}-{key}">
                                                            {label}
                                                            {#if isRequired}<span class="required">*</span>{/if}
                                                        </label>
                                                        <input
                                                            type={property.format === "password" ? "password" : "text"}
                                                            id="config-{action.tempId}-{key}"
                                                            bind:value={action.with[key]}
                                                            oninput={(e) => updateConfigValue(action, key, e.currentTarget.value)}
                                                            {placeholder}
                                                        />
                                                        {#if description}
                                                            <p class="text-lighter field-hint">{description}</p>
                                                        {/if}
                                                    </div>
                                                {/if}
                                            {/each}
                                        </div>
                                    {/if}
                                </div>
                            </div>
                        {/if}

                        <!-- Environment Variables -->
                        <div data-field>
                            <label>Environment Variables</label>
                            <KeyValueEditor
                                bind:pairs={action.variables}
                                keyPlaceholder="VAR_NAME"
                                valuePlaceholder="value OR {'{{'}inputs.name{'}}'}"
                                onchange={() => {}}
                            />
                        </div>

                        <div class="approval-section">
                            <div class="hstack gap-2 items-center">
                                <input
                                    type="checkbox"
                                    id="approval-{action.tempId}"
                                    bind:checked={action.approval}
                                    {disabled}
                                />
                                <label for="approval-{action.tempId}">Require approval before execution</label>
                            </div>

                            {#if action.approval}
                                <div class="approval-details vstack gap-3">
                                    <div data-field>
                                        <label for="approvers-{action.tempId}">Allowed Approvers <span class="hint">(optional — leave blank to allow any reviewer/admin)</span></label>
                                        <input
                                            type="text"
                                            id="approvers-{action.tempId}"
                                            value={(action.approvers || []).join(", ")}
                                            oninput={(e) => {
                                                const raw = e.currentTarget.value;
                                                action.approvers = raw.split(",").map((s: string) => s.trim()).filter(Boolean);
                                            }}
                                            placeholder="alice, bob (comma-separated usernames)"
                                            {disabled}
                                        />
                                        <p class="field-hint text-lighter">Specific users who may approve this step</p>
                                    </div>
                                    <div data-field>
                                        <label for="approval-groups-{action.tempId}">Allowed Approval Groups <span class="hint">(optional)</span></label>
                                        <input
                                            type="text"
                                            id="approval-groups-{action.tempId}"
                                            value={(action.approval_groups || []).join(", ")}
                                            oninput={(e) => {
                                                const raw = e.currentTarget.value;
                                                action.approval_groups = raw.split(",").map((s: string) => s.trim()).filter(Boolean);
                                            }}
                                            placeholder="security-team, ops (comma-separated group names)"
                                            {disabled}
                                        />
                                        <p class="field-hint text-lighter">Groups whose members may approve this step</p>
                                    </div>
                                </div>
                            {/if}
                        </div>
                    </div>
                {/if}
            </div>
        {/each}

        {#if actions.length === 0}
            <div class="empty-state vstack">
                <svg class="empty-icon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
                </svg>
                <p>No actions defined yet</p>
                {#if !disabled}
                    <button
                        data-variant="secondary"
                        onclick={handleAddAction}
                    >
                        Add your first action
                    </button>
                {/if}
            </div>
        {/if}
    </div>
</div>

<style>
    .action-card {
        border: 1px solid var(--border);
        border-radius: 0.5rem;
        overflow: hidden;
        transition: background 0.15s;
    }
    .action-card.drag-over {
        background: var(--faint);
        border-color: var(--primary);
    }
    .action-header {
        background: var(--faint);
        padding: 0.75rem 1rem;
        justify-content: space-between;
        cursor: move;
    }
    .action-name {
        font-weight: 500;
        color: var(--foreground);
    }
    .action-body {
        padding: 1rem;
    }
    .drag-icon {
        width: 1.25rem;
        height: 1.25rem;
        color: var(--muted-foreground);
    }
    .icon-sm {
        width: 1.25rem;
        height: 1.25rem;
    }
    .icon-btn {
        padding: 0.25rem;
        border: none;
        background: none;
    }
    .rotated {
        transform: rotate(180deg);
    }
    .action-badge {
        font-size: 0.75rem;
        padding: 0.125rem 0.5rem;
        background: var(--faint);
        color: var(--foreground);
        border-radius: 9999px;
    }
    .grid-2 {
        display: grid;
        grid-template-columns: 1fr 1fr;
        gap: 1rem;
    }
    .config-section {
        border-top: 1px solid var(--border);
        padding-top: 1rem;
    }
    .config-title {
        font-size: 0.875rem;
        font-weight: 500;
        color: var(--foreground);
        margin-bottom: 0.75rem;
    }
    .required {
        color: var(--danger);
    }
    .field-hint {
        font-size: 0.75rem;
        margin-top: 0.25rem;
    }
    .empty-state {
        text-align: center;
        padding: 3rem 1rem;
        color: var(--muted-foreground);
        border: 2px dashed var(--border);
        border-radius: 0.5rem;
        align-items: center;
        gap: 0.75rem;
    }
    .empty-icon {
        width: 3rem;
        height: 3rem;
        color: var(--muted-foreground);
    }
    .opacity-50 {
        opacity: 0.5;
    }
    .approval-section {
        padding: 0.75rem;
        border: 1px solid var(--border);
        border-radius: 0.375rem;
        background: var(--faint);
    }
    .approval-details {
        margin-top: 0.75rem;
        padding-top: 0.75rem;
        border-top: 1px solid var(--border);
    }
    .hint {
        font-size: 0.7rem;
        font-weight: 400;
        color: var(--muted-foreground);
    }
    .text-lighter {
        color: var(--muted-foreground);
    }
</style>
