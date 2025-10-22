<script lang="ts">
    import { apiClient } from "$lib/apiClient.js";
    import { handleInlineError } from "$lib/utils/errorHandling";
    import { createSlug } from "$lib/utils";
    import CodeEditor from "$lib/components/shared/CodeEditor.svelte";
    import NodeSelector from "$lib/components/shared/NodeSelector.svelte";
    import type { NodeResp } from "$lib/types";

    let {
        namespace,
        actions = $bindable(),
        addAction,
        availableExecutors,
        executorConfigs = $bindable(),
    }: {
        namespace: string;
        actions: any[];
        addAction: () => void;
        availableExecutors: Array<{ name: string; display_name: string }>;
        executorConfigs: Record<string, any>;
    } = $props();
    let draggedIndex: number | null = null;

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

    function addVariable(action: any) {
        if (!action.variables) {
            action.variables = [];
        }
        action.variables.push({ name: "", value: "" });
    }

    function removeVariable(action: any, index: number) {
        action.variables.splice(index, 1);
    }

    function addArtifact(action: any) {
        if (!action.artifacts) {
            action.artifacts = [];
        }
        action.artifacts.push("");
    }

    function removeArtifact(action: any, index: number) {
        action.artifacts.splice(index, 1);
    }

    function updateArtifact(action: any, index: number, value: string) {
        if (!action.artifacts) {
            action.artifacts = [];
        }
        action.artifacts[index] = value.trim();
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
            event.currentTarget.classList.add(
                "bg-primary-50",
                "border-primary-300",
            );
        }
    }

    function dragLeave(event: DragEvent) {
        if (event.currentTarget instanceof HTMLElement) {
            event.currentTarget.classList.remove(
                "bg-primary-50",
                "border-primary-300",
            );
        }
    }

    function drop(event: DragEvent, dropIndex: number) {
        event.preventDefault();
        if (event.currentTarget instanceof HTMLElement) {
            event.currentTarget.classList.remove(
                "bg-primary-50",
                "border-primary-300",
            );
        }
        if (draggedIndex !== null && draggedIndex !== dropIndex) {
            const dragged = actions.splice(draggedIndex, 1)[0];
            actions.splice(dropIndex, 0, dragged);
        }
    }
</script>

<!-- Flow Actions Section -->
<div>
    <div class="flex items-center justify-between mb-6">
        <h3 class="text-base font-medium text-gray-900">Flow Actions</h3>
        <button
            onclick={addAction}
            class="text-sm text-primary-600 hover:text-primary-700 font-medium cursor-pointer"
        >
            + Add Action
        </button>
    </div>

    <div class="space-y-4">
        {#each actions as action, index (action.tempId)}
            <div
                class="border border-gray-200 rounded-lg overflow-hidden transition-colors"
                draggable="true"
                ondragstart={(e) => dragStart(e, index)}
                ondragend={dragEnd}
                ondragover={dragOver}
                ondragleave={dragLeave}
                ondrop={(e) => drop(e, index)}
            >
                <!-- Action Header -->
                <div
                    class="bg-gray-50 px-4 py-3 flex items-center justify-between cursor-move"
                >
                    <div class="flex items-center gap-3">
                        <svg
                            class="w-5 h-5 text-gray-400"
                            fill="none"
                            stroke="currentColor"
                            viewBox="0 0 24 24"
                        >
                            <path
                                stroke-linecap="round"
                                stroke-linejoin="round"
                                stroke-width="2"
                                d="M4 6h16M4 12h16M4 18h16"
                            />
                        </svg>
                        <span class="font-medium text-gray-900"
                            >{action.name || "Untitled Action"}</span
                        >
                        <span
                            class="text-xs px-2 py-1 bg-gray-200 text-gray-700 rounded-full"
                        >
                            {action.executor || "No executor"}
                        </span>
                    </div>
                    <div class="flex items-center gap-2">
                        <button
                            onclick={() =>
                                (action.collapsed = !action.collapsed)}
                            class="text-gray-400 hover:text-gray-600 cursor-pointer"
                        >
                            <svg
                                class="w-5 h-5 transform transition-transform {action.collapsed
                                    ? ''
                                    : 'rotate-180'}"
                                fill="none"
                                stroke="currentColor"
                                viewBox="0 0 24 24"
                            >
                                <path
                                    stroke-linecap="round"
                                    stroke-linejoin="round"
                                    stroke-width="2"
                                    d="M19 9l-7 7-7-7"
                                />
                            </svg>
                        </button>
                        <button
                            onclick={() => duplicateAction(index)}
                            class="text-gray-400 hover:text-primary-600 cursor-pointer"
                        >
                            <svg
                                class="w-5 h-5"
                                fill="none"
                                stroke="currentColor"
                                viewBox="0 0 24 24"
                            >
                                <path
                                    stroke-linecap="round"
                                    stroke-linejoin="round"
                                    stroke-width="2"
                                    d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"
                                />
                            </svg>
                        </button>
                        <button
                            onclick={() => removeAction(index)}
                            class="text-gray-400 hover:text-danger-600 cursor-pointer"
                        >
                            <svg
                                class="w-5 h-5"
                                fill="none"
                                stroke="currentColor"
                                viewBox="0 0 24 24"
                            >
                                <path
                                    stroke-linecap="round"
                                    stroke-linejoin="round"
                                    stroke-width="2"
                                    d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
                                />
                            </svg>
                        </button>
                    </div>
                </div>

                <!-- Action Content -->
                {#if !action.collapsed}
                    <div class="p-4 space-y-4">
                        <!-- Basic Action Fields -->
                        <div class="grid grid-cols-1 gap-4">
                            <div>
                                <label
                                    class="block text-sm font-medium text-gray-700 mb-1"
                                    >Action Name *</label
                                >
                                <input
                                    type="text"
                                    value={action.name}
                                    oninput={(e) =>
                                        updateActionName(
                                            action,
                                            e.currentTarget.value,
                                        )}
                                    class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent text-sm"
                                    placeholder="Action Display Name"
                                />
                            </div>
                        </div>

                        <div class="grid grid-cols-2 gap-4">
                            <div>
                                <label
                                    class="block text-sm font-medium text-gray-700 mb-1"
                                    >Executor *</label
                                >
                                <select
                                    bind:value={action.executor}
                                    onchange={() => onExecutorChange(action)}
                                    class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent text-sm"
                                >
                                    <option value="">Select Executor</option>
                                    {#each availableExecutors as executor}
                                        <option value={executor.name}
                                            >{executor.display_name ||
                                                executor.name}</option
                                        >
                                    {/each}
                                </select>
                            </div>
                            <div>
                                <label
                                    class="block text-sm font-medium text-gray-700 mb-1"
                                    >Run On</label
                                >
                                <NodeSelector
                                    {namespace}
                                    bind:selectedNodes={action.selectedNodes}
                                    placeholder="Search nodes..."
                                />
                            </div>
                        </div>

                        <!-- Dynamic Executor Configuration -->
                        {#if action.executor && executorConfigs[action.executor]}
                            <div class="space-y-4">
                                <div class="border-t pt-4">
                                    <h4
                                        class="text-sm font-medium text-gray-900 mb-3 flex items-center gap-2"
                                    >
                                        <svg
                                            class="w-4 h-4 text-gray-500"
                                            fill="none"
                                            stroke="currentColor"
                                            viewBox="0 0 24 24"
                                        >
                                            <path
                                                stroke-linecap="round"
                                                stroke-linejoin="round"
                                                stroke-width="2"
                                                d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"
                                            />
                                            <path
                                                stroke-linecap="round"
                                                stroke-linejoin="round"
                                                stroke-width="2"
                                                d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"
                                            />
                                        </svg>
                                        <span
                                            >{action.executor
                                                .charAt(0)
                                                .toUpperCase() +
                                                action.executor.slice(1)}</span
                                        >
                                        Configuration
                                    </h4>

                                    <!-- Dynamic form fields based on JSON schema -->
                                    {#if executorConfigs[action.executor].properties}
                                        <div class="space-y-4">
                                            {#each Object.entries(executorConfigs[action.executor].properties) as [key, property]}
                                                {@const isRequired =
                                                    executorConfigs[
                                                        action.executor
                                                    ].required?.includes(key)}
                                                {@const label =
                                                    property.title || key}
                                                {@const description =
                                                    property.description || ""}
                                                {@const placeholder =
                                                    property.examples?.[0] ||
                                                    property.default ||
                                                    ""}

                                                {#if property.type === "boolean"}
                                                    <div
                                                        class="flex items-start"
                                                    >
                                                        <input
                                                            type="checkbox"
                                                            id="config-{action.tempId}-{key}"
                                                            bind:checked={
                                                                action.with[key]
                                                            }
                                                            onchange={(e) =>
                                                                updateConfigValue(
                                                                    action,
                                                                    key,
                                                                    e
                                                                        .currentTarget
                                                                        .checked,
                                                                )}
                                                            class="h-4 w-4 text-primary-600 focus:ring-primary-500 border-gray-300 rounded mt-0.5"
                                                        />
                                                        <div class="ml-2">
                                                            <label
                                                                for="config-{action.tempId}-{key}"
                                                                class="text-sm text-gray-700"
                                                            >
                                                                {label}
                                                                {#if isRequired}<span
                                                                        class="text-red-500"
                                                                        >*</span
                                                                    >{/if}
                                                            </label>
                                                            {#if description}
                                                                <p
                                                                    class="text-xs text-gray-500 mt-1"
                                                                >
                                                                    {description}
                                                                </p>
                                                            {/if}
                                                        </div>
                                                    </div>
                                                {:else if property.enum}
                                                    <div>
                                                        <label
                                                            for="config-{action.tempId}-{key}"
                                                            class="block text-sm font-medium text-gray-700 mb-1"
                                                        >
                                                            {label}
                                                            {#if isRequired}<span
                                                                    class="text-red-500"
                                                                    >*</span
                                                                >{/if}
                                                        </label>
                                                        <select
                                                            id="config-{action.tempId}-{key}"
                                                            bind:value={
                                                                action.with[key]
                                                            }
                                                            onchange={(e) =>
                                                                updateConfigValue(
                                                                    action,
                                                                    key,
                                                                    e
                                                                        .currentTarget
                                                                        .value,
                                                                )}
                                                            class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent text-sm"
                                                        >
                                                            <option value=""
                                                                >Select...</option
                                                            >
                                                            {#each property.enum as option}
                                                                <option
                                                                    value={option}
                                                                    >{option}</option
                                                                >
                                                            {/each}
                                                        </select>
                                                        {#if description}
                                                            <p
                                                                class="mt-1 text-xs text-gray-500"
                                                            >
                                                                {description}
                                                            </p>
                                                        {/if}
                                                    </div>
                                                {:else if property.type === "number" || property.type === "integer"}
                                                    <div>
                                                        <label
                                                            for="config-{action.tempId}-{key}"
                                                            class="block text-sm font-medium text-gray-700 mb-1"
                                                        >
                                                            {label}
                                                            {#if isRequired}<span
                                                                    class="text-red-500"
                                                                    >*</span
                                                                >{/if}
                                                        </label>
                                                        <input
                                                            type="number"
                                                            id="config-{action.tempId}-{key}"
                                                            bind:value={
                                                                action.with[key]
                                                            }
                                                            oninput={(e) =>
                                                                updateConfigValue(
                                                                    action,
                                                                    key,
                                                                    property.type ===
                                                                        "integer"
                                                                        ? parseInt(
                                                                              e
                                                                                  .currentTarget
                                                                                  .value,
                                                                          )
                                                                        : parseFloat(
                                                                              e
                                                                                  .currentTarget
                                                                                  .value,
                                                                          ),
                                                                )}
                                                            step={property.type ===
                                                            "integer"
                                                                ? "1"
                                                                : "any"}
                                                            min={property.minimum}
                                                            max={property.maximum}
                                                            {placeholder}
                                                            class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent text-sm"
                                                        />
                                                        {#if description}
                                                            <p
                                                                class="mt-1 text-xs text-gray-500"
                                                            >
                                                                {description}
                                                            </p>
                                                        {/if}
                                                    </div>
                                                {:else if property.widget === "codeeditor"}
                                                    <div>
                                                        <label
                                                            class="block text-sm font-medium text-gray-700 mb-1"
                                                        >
                                                            {label}
                                                            {#if isRequired}<span
                                                                    class="text-red-500"
                                                                    >*</span
                                                                >{/if}
                                                        </label>
                                                        <CodeEditor
                                                            value={action.with[
                                                                key
                                                            ] || ""}
                                                            language={property.language ||
                                                                "python"}
                                                            height="200px"
                                                            onchange={(val) =>
                                                                updateConfigValue(
                                                                    action,
                                                                    key,
                                                                    val,
                                                                )}
                                                        />
                                                        {#if description}
                                                            <p
                                                                class="mt-1 text-xs text-gray-500"
                                                            >
                                                                {description}
                                                            </p>
                                                        {/if}
                                                    </div>
                                                {:else if property.format === "textarea" || property.type === "object" || property.type === "array"}
                                                    <div>
                                                        <label
                                                            for="config-{action.tempId}-{key}"
                                                            class="block text-sm font-medium text-gray-700 mb-1"
                                                        >
                                                            {label}
                                                            {#if isRequired}<span
                                                                    class="text-red-500"
                                                                    >*</span
                                                                >{/if}
                                                        </label>
                                                        <textarea
                                                            id="config-{action.tempId}-{key}"
                                                            bind:value={
                                                                action.with[key]
                                                            }
                                                            oninput={(e) =>
                                                                updateConfigValue(
                                                                    action,
                                                                    key,
                                                                    e
                                                                        .currentTarget
                                                                        .value,
                                                                )}
                                                            placeholder={placeholder ||
                                                                (property.type ===
                                                                "object"
                                                                    ? "JSON object"
                                                                    : property.type ===
                                                                        "array"
                                                                      ? "Array values"
                                                                      : "Multi-line text")}
                                                            rows="4"
                                                            class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent text-sm font-mono"
                                                        ></textarea>
                                                        {#if description}
                                                            <p
                                                                class="mt-1 text-xs text-gray-500"
                                                            >
                                                                {description}
                                                            </p>
                                                        {/if}
                                                        {#if property.type === "object" || property.type === "array"}
                                                            <p
                                                                class="mt-1 text-xs text-gray-400"
                                                            >
                                                                Enter as JSON
                                                                format
                                                            </p>
                                                        {/if}
                                                    </div>
                                                {:else}
                                                    <div>
                                                        <label
                                                            for="config-{action.tempId}-{key}"
                                                            class="block text-sm font-medium text-gray-700 mb-1"
                                                        >
                                                            {label}
                                                            {#if isRequired}<span
                                                                    class="text-red-500"
                                                                    >*</span
                                                                >{/if}
                                                        </label>
                                                        <input
                                                            type={property.format ===
                                                            "password"
                                                                ? "password"
                                                                : "text"}
                                                            id="config-{action.tempId}-{key}"
                                                            bind:value={
                                                                action.with[key]
                                                            }
                                                            oninput={(e) =>
                                                                updateConfigValue(
                                                                    action,
                                                                    key,
                                                                    e
                                                                        .currentTarget
                                                                        .value,
                                                                )}
                                                            {placeholder}
                                                            class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent text-sm"
                                                        />
                                                        {#if description}
                                                            <p
                                                                class="mt-1 text-xs text-gray-500"
                                                            >
                                                                {description}
                                                            </p>
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
                        <div>
                            <div class="flex items-center justify-between mb-2">
                                <label
                                    class="block text-sm font-medium text-gray-700"
                                    >Environment Variables</label
                                >
                                <button
                                    onclick={() => addVariable(action)}
                                    type="button"
                                    class="text-xs text-primary-600 hover:text-primary-700 cursor-pointer"
                                >
                                    + Add Variable
                                </button>
                            </div>
                            <div class="space-y-2">
                                {#each (action.variables && action.variables.length > 0) ? action.variables : [{ name: "", value: "" }] as variable, varIndex}
                                    <div class="flex items-center gap-2">
                                        <input
                                            type="text"
                                            value={variable.name}
                                            oninput={(e) => {
                                                if (!action.variables || action.variables.length === 0) {
                                                    action.variables = [{ name: "", value: "" }];
                                                }
                                                action.variables[varIndex].name = e.currentTarget.value;
                                            }}
                                            placeholder="VAR_NAME"
                                            class="flex-1 px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent text-sm font-mono"
                                        />
                                        <span class="text-gray-500">=</span>
                                        <input
                                            type="text"
                                            value={variable.value}
                                            oninput={(e) => {
                                                if (!action.variables || action.variables.length === 0) {
                                                    action.variables = [{ name: "", value: "" }];
                                                }
                                                action.variables[varIndex].value = e.currentTarget.value;
                                            }}
                                            class="flex-1 px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent text-sm font-mono"
                                        />
                                        {#if action.variables && action.variables.length > 0}
                                            <button
                                                onclick={() =>
                                                    removeVariable(
                                                        action,
                                                        varIndex,
                                                    )}
                                                type="button"
                                                class="text-gray-400 hover:text-danger-600 cursor-pointer"
                                            >
                                                <svg
                                                    class="w-4 h-4"
                                                    fill="none"
                                                    stroke="currentColor"
                                                    viewBox="0 0 24 24"
                                                >
                                                    <path
                                                        stroke-linecap="round"
                                                        stroke-linejoin="round"
                                                        stroke-width="2"
                                                        d="M6 18L18 6M6 6l12 12"
                                                    />
                                                </svg>
                                            </button>
                                        {:else}
                                            <div class="w-4"></div>
                                        {/if}
                                    </div>
                                {/each}
                            </div>
                        </div>

                        <!-- Additional Options -->
                        <div class="grid grid-cols-2 gap-4">
                            <div>
                                <div
                                    class="flex items-center justify-between mb-2"
                                >
                                    <label
                                        class="block text-sm font-medium text-gray-700"
                                        >Artifacts</label
                                    >
                                    <button
                                        onclick={() => addArtifact(action)}
                                        type="button"
                                        class="text-xs text-primary-600 hover:text-primary-700 cursor-pointer"
                                    >
                                        + Add Artifact
                                    </button>
                                </div>
                                <div class="space-y-2">
                                    {#each (action.artifacts && action.artifacts.length > 0) ? action.artifacts : [""] as artifact, artifactIndex}
                                        <div class="flex items-center gap-2">
                                            <input
                                                type="text"
                                                value={artifact}
                                                oninput={(e) => {
                                                    if (!action.artifacts || action.artifacts.length === 0) {
                                                        action.artifacts = [""];
                                                    }
                                                    updateArtifact(
                                                        action,
                                                        artifactIndex,
                                                        e.currentTarget.value,
                                                    );
                                                }}
                                                placeholder="/path/to/output/file"
                                                class="flex-1 px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent text-sm font-mono"
                                            />
                                            {#if action.artifacts && action.artifacts.length > 0}
                                                <button
                                                    onclick={() =>
                                                        removeArtifact(
                                                            action,
                                                            artifactIndex,
                                                        )}
                                                    type="button"
                                                    class="text-gray-400 hover:text-danger-600 cursor-pointer"
                                                >
                                                    <svg
                                                        class="w-4 h-4"
                                                        fill="none"
                                                        stroke="currentColor"
                                                        viewBox="0 0 24 24"
                                                    >
                                                        <path
                                                            stroke-linecap="round"
                                                            stroke-linejoin="round"
                                                            stroke-width="2"
                                                            d="M6 18L18 6M6 6l12 12"
                                                        />
                                                    </svg>
                                                </button>
                                            {:else}
                                                <div class="w-4"></div>
                                            {/if}
                                        </div>
                                    {/each}
                                </div>
                                <p class="mt-1 text-xs text-gray-500">
                                    File paths to preserve as execution
                                    artifacts
                                </p>
                            </div>
                        </div>

                        <div class="flex items-center">
                            <input
                                type="checkbox"
                                bind:checked={action.approval}
                                class="h-4 w-4 text-primary-600 focus:ring-primary-500 border-gray-300 rounded"
                            />
                            <label class="ml-2 block text-sm text-gray-700"
                                >Require approval before execution</label
                            >
                        </div>
                    </div>
                {/if}
            </div>
        {/each}

        {#if actions.length === 0}
            <div
                class="text-center py-12 text-gray-500 border-2 border-dashed border-gray-300 rounded-lg"
            >
                <svg
                    class="mx-auto h-12 w-12 text-gray-400 mb-3"
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                >
                    <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="2"
                        d="M13 10V3L4 14h7v7l9-11h-7z"
                    />
                </svg>
                <p>No actions defined yet</p>
                <button
                    onclick={addAction}
                    class="mt-2 text-sm text-primary-600 hover:text-primary-700 font-medium cursor-pointer"
                >
                    Add your first action
                </button>
            </div>
        {/if}
    </div>
</div>
