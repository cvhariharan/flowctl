<script lang="ts">
    import OatSelect from "$lib/components/shared/OatSelect.svelte";

    let {
        inputs = $bindable(),
        addInput,
        disabled = false,
    }: {
        inputs: any[];
        addInput: () => void;
        disabled?: boolean;
    } = $props();

    // Keep max_file_size in sync with maxFileSizeMB for file inputs
    $effect(() => {
        for (const input of inputs) {
            if (input.type === 'file') {
                const mb = input.maxFileSizeMB;
                input.max_file_size = mb ? mb * 1024 * 1024 : undefined;
            }
        }
    });

    function removeInput(index: number) {
        inputs.splice(index, 1);
    }

    function sanitizeName(value: string) {
        return value.replace(/[^a-zA-Z0-9_]/g, "");
    }

    function onInputTypeChange(input: any) {
        if (input.type !== "select") {
            input.options = [];
            input.optionsText = "";
            input.useRemoteOptions = false;
            input.remote_options = undefined;
        }
        if (input.type === "file") {
            input.default = "";
        }
        if (input.type !== "file") {
            input.max_file_size = undefined;
            input.maxFileSizeMB = undefined;
        }
        if (input.type === "node" && input.multiple === undefined) {
            input.multiple = false;
        }
    }

    function updateOptions(input: any) {
        input.options = input.optionsText
            .split("\n")
            .filter((opt: string) => opt.trim());
    }

    function toggleRemoteOptions(input: any) {
        input.useRemoteOptions = !input.useRemoteOptions;
        if (input.useRemoteOptions) {
            input.remote_options = input.remote_options ?? { url: "", method: "GET", headers: {} };
            input.options = [];
            input.optionsText = "";
        } else {
            input.remote_options = undefined;
        }
    }

    function addRemoteHeader(input: any) {
        if (!input.remote_options.headers) {
            input.remote_options.headers = {};
        }
        if (!input.remoteHeaders) {
            input.remoteHeaders = [];
        }
        input.remoteHeaders = [...input.remoteHeaders, { key: "", value: "" }];
    }

    function removeRemoteHeader(input: any, index: number) {
        input.remoteHeaders.splice(index, 1);
        input.remoteHeaders = [...input.remoteHeaders];
        syncHeaders(input);
    }

    function syncHeaders(input: any) {
        const headers: Record<string, string> = {};
        for (const h of (input.remoteHeaders ?? [])) {
            if (h.key.trim()) {
                headers[h.key.trim()] = h.value;
            }
        }
        input.remote_options.headers = headers;
    }
</script>

<!-- Flow Inputs Section -->
<div>
    <div class="hstack mb-4 justify-between">
        <h3>Flow Inputs</h3>
        {#if !disabled}
            <button onclick={addInput}>
                + Add Input
            </button>
        {/if}
    </div>

    <div class="vstack gap-4">
        {#each inputs as input, index (index)}
            <div class="input-card">
                {#if !disabled}
                <button
                    type="button"
                    data-variant="danger"
                    class="ghost icon small remove-btn"
                    onclick={() => removeInput(index)}
                >
                    <svg class="icon-sm" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                    </svg>
                </button>
                {/if}

                <div class="grid-3">
                    <div data-field>
                        <label>Input Name *</label>
                        <input
                            type="text"
                            bind:value={input.name}
                            oninput={(e) => (input.name = sanitizeName(e.currentTarget.value))}
                            placeholder="input_name"
                        />
                    </div>
                    <div data-field>
                        <label>Type *</label>
                        <OatSelect
                            bind:value={input.type}
                            options={[
                                { value: 'string', label: 'String' },
                                { value: 'number', label: 'Number' },
                                { value: 'checkbox', label: 'Checkbox' },
                                { value: 'password', label: 'Password' },
                                { value: 'file', label: 'File' },
                                { value: 'datetime', label: 'DateTime' },
                                { value: 'select', label: 'Select' },
                                { value: 'node', label: 'Node' }
                            ]}
                            onchange={() => onInputTypeChange(input)}
                        />
                    </div>
                    <div data-field>
                        <label>Label</label>
                        <input
                            type="text"
                            bind:value={input.label}
                            placeholder="Display Label"
                        />
                    </div>
                    <div class="col-span-2" data-field>
                        <label>Description</label>
                        <input
                            type="text"
                            bind:value={input.description}
                            placeholder="Help text for this input"
                        />
                    </div>
                    <div data-field>
                        <label class:text-lighter={input.type === "file"}>Default Value</label>
                        <input
                            type="text"
                            bind:value={input.default}
                            disabled={input.type === "file"}
                            placeholder={input.type === "file" ? "Not available for file inputs" : "Default value"}
                        />
                    </div>
                    <div class="col-span-2" data-field>
                        <label>Validation</label>
                        <input
                            type="text"
                            bind:value={input.validation}
                            class="mono"
                            placeholder="len(input_name) > 3"
                        />
                    </div>
                    <div class="hstack gap-2">
                        <input
                            type="checkbox"
                            bind:checked={input.required}
                        />
                        <label>Required</label>
                    </div>
                </div>

                {#if input.type === "select"}
                    <div class="options-section">
                        <div class="hstack justify-between">
                            <span>Options Source</span>
                            <div class="hstack gap-2">
                                <span class="text-lighter">Static</span>
                                <input
                                    type="checkbox"
                                    role="switch"
                                    checked={input.useRemoteOptions ?? false}
                                    onchange={() => toggleRemoteOptions(input)}
                                />
                                <span class="text-lighter">Remote API</span>
                            </div>
                        </div>

                        {#if input.useRemoteOptions}
                            <div class="vstack gap-2 mt-2">
                                <div class="grid-4">
                                    <div data-field>
                                        <label>Method</label>
                                        <OatSelect
                                            bind:value={input.remote_options.method}
                                            options={[
                                                { value: 'GET', label: 'GET' },
                                                { value: 'POST', label: 'POST' }
                                            ]}
                                        />
                                    </div>
                                    <div class="col-span-3" data-field>
                                        <label>URL *</label>
                                        <input
                                            type="url"
                                            bind:value={input.remote_options.url}
                                            class="mono"
                                            placeholder="https://api.example.com/options"
                                        />
                                    </div>
                                </div>

                                <div>
                                    <div class="hstack justify-between mb-2">
                                        <label>Headers</label>
                                        <button
                                            type="button"
                                            data-variant="secondary"
                                            class="add-header-btn"
                                            onclick={() => addRemoteHeader(input)}
                                        >
                                            + Add Header
                                        </button>
                                    </div>
                                    {#if input.remoteHeaders && input.remoteHeaders.length > 0}
                                        <div class="vstack gap-1">
                                            {#each input.remoteHeaders as header, hi (hi)}
                                                <div class="hstack gap-2">
                                                    <input
                                                        type="text"
                                                        bind:value={header.key}
                                                        oninput={() => syncHeaders(input)}
                                                        class="mono"
                                                        placeholder="Header name"
                                                    />
                                                    <input
                                                        type="text"
                                                        bind:value={header.value}
                                                        oninput={() => syncHeaders(input)}
                                                        class="mono"
                                                        placeholder="Value or {'{{ secrets.KEY }}'}"
                                                    />
                                                    <button
                                                        type="button"
                                                        data-variant="danger"
                                                        class="ghost icon small"
                                                        onclick={() => removeRemoteHeader(input, hi)}
                                                    >
                                                        <svg class="icon-sm" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                                                        </svg>
                                                    </button>
                                                </div>
                                            {/each}
                                        </div>
                                    {:else}
                                        <p class="text-lighter hint">No headers. Use <code>{'{{ secrets.KEY }}'}</code> for interpolation.</p>
                                    {/if}
                                </div>
                            </div>
                        {:else}
                            <div data-field class="mt-2">
                                <label>Options (one per line)</label>
                                <textarea
                                    bind:value={input.optionsText}
                                    oninput={() => updateOptions(input)}
                                    class="mono options-textarea"
                                    placeholder="option1&#10;option2&#10;option3"
                                ></textarea>
                            </div>
                        {/if}
                    </div>
                {/if}

                {#if input.type === "node"}
                    <div class="options-section mt-4">
                        <div class="hstack gap-2">
                            <input
                                type="checkbox"
                                bind:checked={input.multiple}
                            />
                            <label>Allow selecting multiple nodes/tags</label>
                        </div>
                    </div>
                {/if}

                {#if input.type === "file"}
                    <div class="options-section mt-4">
                        <div data-field>
                            <label>Max File Size (MB)</label>
                            <input
                                type="number"
                                bind:value={input.maxFileSizeMB}
                                placeholder="Leave empty for default (100MB)"
                                min="1"
                            />
                            <p class="text-lighter hint">Optional. Leave empty to use server default.</p>
                        </div>
                    </div>
                {/if}
            </div>
        {/each}

        {#if inputs.length === 0}
            <div class="empty-state vstack">
                <svg class="empty-icon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 13h6m-3-3v6m5 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
                </svg>
                <p>No inputs defined yet</p>
                {#if !disabled}
                    <button data-variant="secondary" onclick={addInput}>
                        Add your first input
                    </button>
                {/if}
            </div>
        {/if}
    </div>
</div>

<style>
    .input-card {
        border: 1px solid var(--border);
        border-radius: 0.5rem;
        padding: 1rem;
        position: relative;
    }
    .remove-btn {
        position: absolute;
        top: var(--space-2);
        right: var(--space-2);
        z-index: 1;
    }
    .grid-3 {
        display: grid;
        grid-template-columns: 1fr 1fr 1fr;
        gap: 1rem;
    }
    .grid-4 {
        display: grid;
        grid-template-columns: 1fr 3fr;
        gap: 0.5rem;
    }
    .col-span-2 {
        grid-column: span 2;
    }
    .col-span-3 {
        grid-column: span 3;
    }
    .mono {
        font-family: monospace;
    }
    .icon-sm {
        width: 1.25rem;
        height: 1.25rem;
    }
    .icon-btn {
        padding: 0.25rem;
        border: none;
        background: none;
        flex-shrink: 0;
    }
    .options-section {
        margin-top: 1rem;
        padding: 0.75rem;
        background: var(--faint);
        border-radius: 0.375rem;
    }
    .options-textarea {
        height: 5rem;
    }
    .add-header-btn {
        font-size: 0.75rem;
        padding: 0.125rem 0.5rem;
        border: none;
        background: none;
    }
    .hint {
        font-size: 0.75rem;
        margin-top: 0.25rem;
    }
    .mt-2 { margin-top: 0.5rem; }
    .mt-4 { margin-top: 1rem; }
    .empty-state {
        text-align: center;
        padding: 2rem 1rem;
        color: var(--muted-foreground);
        align-items: center;
        gap: 0.75rem;
    }
    .empty-icon {
        width: 3rem;
        height: 3rem;
        color: var(--muted-foreground);
    }
</style>
