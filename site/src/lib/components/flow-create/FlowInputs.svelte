<script lang="ts">
    let {
        inputs = $bindable(),
        addInput,
    }: {
        inputs: any[];
        addInput: () => void;
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
        }
        if (input.type === "file") {
            input.default = "";
        }
        if (input.type !== "file") {
            input.max_file_size = undefined;
            input.maxFileSizeMB = undefined;
        }
    }

    function updateOptions(input: any) {
        input.options = input.optionsText
            .split("\n")
            .filter((opt: string) => opt.trim());
    }
</script>

<!-- Flow Inputs Section -->
<div>
    <div class="flex items-center justify-between mb-6">
        <h3 class="text-base font-medium text-foreground">Flow Inputs</h3>
        <button
            onclick={addInput}
            class="px-4 py-2 text-sm font-medium bg-primary-500 text-white rounded-md hover:bg-primary-600 focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2 cursor-pointer"
        >
            + Add Input
        </button>
    </div>

    <div class="space-y-4">
        {#each inputs as input, index (index)}
            <div class="border border-border rounded-lg p-4 relative">
                <button
                    onclick={() => removeInput(index)}
                    class="absolute top-4 right-4 text-muted-foreground hover:text-danger-600 cursor-pointer"
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
                            d="M6 18L18 6M6 6l12 12"
                        />
                    </svg>
                </button>

                <div class="grid grid-cols-3 gap-4">
                    <div>
                        <label
                            class="block text-sm font-medium text-foreground mb-1"
                            >Input Name *</label
                        >
                        <input
                            type="text"
                            bind:value={input.name}
                            oninput={(e) =>
                                (input.name = sanitizeName(
                                    e.currentTarget.value,
                                ))}
                            class="w-full px-3 py-2 text-foreground bg-card border border-input rounded-md focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent text-sm"
                            placeholder="input_name"
                            required
                        />
                    </div>
                    <div>
                        <label
                            class="block text-sm font-medium text-foreground mb-1"
                            >Type *</label
                        >
                        <select
                            bind:value={input.type}
                            onchange={() => onInputTypeChange(input)}
                            class="w-full px-3 py-2 text-foreground bg-card border border-input rounded-md focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent text-sm"
                            required
                        >
                            <option value="string">String</option>
                            <option value="number">Number</option>
                            <option value="checkbox">Checkbox</option>
                            <option value="password">Password</option>
                            <option value="file">File</option>
                            <option value="datetime">DateTime</option>
                            <option value="select">Select</option>
                        </select>
                    </div>
                    <div>
                        <label
                            class="block text-sm font-medium text-foreground mb-1"
                            >Label</label
                        >
                        <input
                            type="text"
                            bind:value={input.label}
                            class="w-full px-3 py-2 text-foreground bg-card border border-input rounded-md focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent text-sm"
                            placeholder="Display Label"
                        />
                    </div>
                    <div class="col-span-2">
                        <label
                            class="block text-sm font-medium text-foreground mb-1"
                            >Description</label
                        >
                        <input
                            type="text"
                            bind:value={input.description}
                            class="w-full px-3 py-2 text-foreground bg-card border border-input rounded-md focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent text-sm"
                            placeholder="Help text for this input"
                        />
                    </div>
                    <div>
                        <label
                            class="block text-sm font-medium mb-1"
                            class:text-foreground={input.type !== "file"}
                            class:text-muted-foreground={input.type === "file"}
                            >Default Value</label
                        >
                        <input
                            type="text"
                            bind:value={input.default}
                            disabled={input.type === "file"}
                            class="w-full px-3 py-2 text-foreground bg-card border border-input rounded-md focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent text-sm disabled:bg-subtle disabled:text-muted-foreground disabled:cursor-not-allowed"
                            placeholder={input.type === "file" ? "Not available for file inputs" : "Default value"}
                        />
                    </div>
                    <div class="col-span-2">
                        <label
                            class="block text-sm font-medium text-foreground mb-1"
                            >Validation</label
                        >
                        <input
                            type="text"
                            bind:value={input.validation}
                            class="w-full px-3 py-2 text-foreground bg-card border border-input rounded-md focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent text-sm font-mono"
                            placeholder="len(input_name) > 3"
                        />
                    </div>
                    <div class="flex items-center">
                        <input
                            type="checkbox"
                            bind:checked={input.required}
                            class="h-4 w-4 text-primary-600 focus:ring-primary-500 border-input rounded"
                        />
                        <label class="ml-2 block text-sm text-foreground"
                            >Required</label
                        >
                    </div>
                </div>

                {#if input.type === "select"}
                    <div class="mt-4 p-3 bg-muted rounded-md space-y-4">
                        <div>
                            <label
                                class="block text-sm font-medium text-foreground mb-2"
                                >Options (one per line)</label
                            >
                            <textarea
                                bind:value={input.optionsText}
                                oninput={() => updateOptions(input)}
                                class="w-full px-3 py-2 text-foreground bg-card border border-input rounded-md focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent text-sm font-mono h-20"
                                placeholder="option1&#10;option2&#10;option3"
                            ></textarea>
                            <p class="text-xs text-muted-foreground mt-1">Static options (leave empty if using remote URL)</p>
                        </div>
                        <div>
                            <label
                                class="block text-sm font-medium text-foreground mb-2"
                                >Remote Options URL</label
                            >
                            <input
                                type="text"
                                bind:value={input.options_url}
                                class="w-full px-3 py-2 text-foreground bg-card border border-input rounded-md focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent text-sm font-mono"
                                placeholder="https://api.example.com/options?env={{environment}}"
                            />
                            <p class="text-xs text-muted-foreground mt-1">JSON API endpoint returning array of objects with 'name' field. Supports variable interpolation using {{'{{'}}variable_name{{'}}'}}</p>
                        </div>
                    </div>
                {/if}

                {#if input.type === "file"}
                    <div class="mt-4 p-3 bg-muted rounded-md">
                        <label
                            class="block text-sm font-medium text-foreground mb-2"
                            >Max File Size (MB)</label
                        >
                        <input
                            type="number"
                            bind:value={input.maxFileSizeMB}
                            class="w-full px-3 py-2 text-foreground bg-card border border-input rounded-md focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent text-sm"
                            placeholder="Leave empty for default (100MB)"
                            min="1"
                        />
                        <p class="text-xs text-muted-foreground mt-1">Optional. Leave empty to use server default.</p>
                    </div>
                {/if}
            </div>
        {/each}

        {#if inputs.length === 0}
            <div class="text-center py-8 text-muted-foreground">
                <svg
                    class="mx-auto h-12 w-12 text-muted-foreground mb-3"
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                >
                    <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="2"
                        d="M9 13h6m-3-3v6m5 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
                    />
                </svg>
                <p>No inputs defined yet</p>
                <button
                    onclick={addInput}
                    class="mt-2 text-sm text-primary-600 hover:text-primary-700 font-medium cursor-pointer"
                >
                    Add your first input
                </button>
            </div>
        {/if}
    </div>
</div>
