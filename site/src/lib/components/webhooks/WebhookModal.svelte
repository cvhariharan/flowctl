<script lang="ts">
    import { handleInlineError } from "$lib/utils/errorHandling";
    import { autofocus } from "$lib/utils/autofocus";
    import type {
        WebhookCreateReq,
        WebhookUpdateReq,
        WebhookResp,
        WebhookHeader,
    } from "$lib/types";

    interface HeaderRow extends WebhookHeader {
        masked: boolean;
    }

    interface Props {
        isEditMode?: boolean;
        webhookData?: WebhookResp | null;
        onSave: (data: WebhookCreateReq | WebhookUpdateReq) => void;
        onClose: () => void;
        onTest?: () => Promise<void>;
    }

    let {
        isEditMode = false,
        webhookData = null,
        onSave,
        onClose,
        onTest,
    }: Props = $props();

    const presets = {
        slack: {
            format: "slack",
            contentType: "application/json",
            body: `{"text":"Flow {{ .flow.name }} {{ .execution.status }} in {{ .namespace.name }}"}`
        },
        teams: {
            format: "teams",
            contentType: "application/json",
            body: `{"@type":"MessageCard","@context":"http://schema.org/extensions","summary":"Flow {{ .flow.name }}","themeColor":"0076D7","title":"Flow {{ .flow.name }}","sections":[{"activityTitle":"{{ .execution.status }}","facts":[{"name":"Execution","value":"{{ .execution.id }}"}]}]}`
        },
        generic: {
            format: "json",
            contentType: "application/json",
            body: `{"text":"Flow {{ .flow.name }} {{ .execution.status }}"}`
        },
    } as const;

    const variables = [
        { token: "{{ .flow.id }}", label: "flow.id", description: "Flow ID" },
        { token: "{{ .flow.name }}", label: "flow.name", description: "Flow name" },
        { token: "{{ .flow.slug }}", label: "flow.slug", description: "Flow slug" },
        { token: "{{ .flow.url }}", label: "flow.url", description: "Flow URL" },
        { token: "{{ .execution.id }}", label: "execution.id", description: "Execution ID" },
        { token: "{{ .execution.status }}", label: "execution.status", description: "Execution status" },
        { token: "{{ .execution.trigger_type }}", label: "execution.trigger_type", description: "Trigger type" },
        { token: "{{ .execution.triggered_by }}", label: "execution.triggered_by", description: "Triggered by" },
        { token: "{{ .execution.started_at }}", label: "execution.started_at", description: "Started at" },
        { token: "{{ .execution.completed_at }}", label: "execution.completed_at", description: "Completed at" },
        { token: "{{ .execution.error }}", label: "execution.error", description: "Error message" },
        { token: "{{ .namespace.id }}", label: "namespace.id", description: "Namespace ID" },
        { token: "{{ .namespace.name }}", label: "namespace.name", description: "Namespace name" },
        { token: "{{ .app.root_url }}", label: "app.root_url", description: "Root URL" },
    ];

    let formData = $state({
        name: "",
        description: "",
        type: "generic",
        url: "",
        contentType: "application/json",
        templateFormat: "json",
        templateBody: presets.generic.body,
        isActive: true,
    });

    let headers = $state<HeaderRow[]>([]);
    let urlMasked = $state("");
    let urlTouched = $state(false);
    let headersTouched = $state(false);
    let templateTouched = $state(false);
    let contentTypeTouched = $state(false);
    let loading = $state(false);
    let testing = $state(false);
    let formError = $state("");
    let previewOpen = $state(false);

    const sampleData = {
        flow: {
            id: "sample-flow",
            name: "Sample Flow",
            slug: "sample-flow",
            url: "https://app.flowctl.local/view/default/flows/sample-flow",
        },
        execution: {
            id: "exec-sample-123",
            status: "completed",
            trigger_type: "manual",
            triggered_by: "system@example.com",
            started_at: "2024-01-01T00:00:00Z",
            completed_at: "2024-01-01T00:02:00Z",
            error: "",
        },
        namespace: {
            id: "00000000-0000-0000-0000-000000000000",
            name: "default",
        },
        app: {
            root_url: "https://app.flowctl.local",
        },
    };

    $effect(() => {
        if (isEditMode && webhookData) {
            formData = {
                name: webhookData.name || "",
                description: webhookData.description || "",
                type: webhookData.type || "generic",
                url: "",
                contentType: webhookData.content_type || "application/json",
                templateFormat: webhookData.template?.format || "json",
                templateBody: webhookData.template?.body || presets.generic.body,
                isActive: webhookData.is_active ?? true,
            };
            urlMasked = webhookData.url_masked || "";
            headers = (webhookData.headers || []).map((header) => ({
                key: header.key,
                value: "",
                masked: header.value === "********",
            }));
            urlTouched = false;
            headersTouched = false;
            templateTouched = false;
            contentTypeTouched = false;
            formError = "";
        } else if (!isEditMode) {
            applyPreset("generic", true);
            headers = [];
            urlMasked = "";
            urlTouched = false;
            headersTouched = false;
            templateTouched = false;
            contentTypeTouched = false;
            formError = "";
        }
    });

    function applyPreset(type: string, force = false) {
        const preset = presets[type as keyof typeof presets] || presets.generic;
        if (!templateTouched || force) {
            formData.templateBody = preset.body;
            formData.templateFormat = preset.format;
        }
        if (!contentTypeTouched || force) {
            formData.contentType = preset.contentType;
        }
    }

    $effect(() => {
        if (!isEditMode) {
            applyPreset(formData.type);
        }
    });

    function handleTypeChange(event: Event) {
        const target = event.target as HTMLSelectElement;
        formData.type = target.value;
        applyPreset(formData.type);
    }

    function handleUrlInput(event: Event) {
        urlTouched = true;
        formData.url = (event.target as HTMLInputElement).value;
    }

    function addHeader() {
        headers = [...headers, { key: "", value: "", masked: false }];
        headersTouched = true;
    }

    function removeHeader(index: number) {
        headers = headers.filter((_, idx) => idx !== index);
        headersTouched = true;
    }

    function updateHeader(index: number, field: "key" | "value", value: string) {
        headers[index] = { ...headers[index], [field]: value, masked: false };
        headersTouched = true;
    }

    function insertVariable(token: string) {
        formData.templateBody = `${formData.templateBody}\n${token}`.trim();
        templateTouched = true;
    }

    function renderPreview(body: string): string {
        return body.replace(/{{\s*\.([a-z_]+)\.([a-z_]+)\s*}}/gi, (_match, group, key) => {
            const section = (sampleData as any)[group];
            if (!section) return "";
            const value = section[key];
            return value ?? "";
        });
    }

    function buildPayload(): WebhookCreateReq | WebhookUpdateReq | null {
        formError = "";

        if (!formData.name.trim()) {
            formError = "Name is required.";
            return null;
        }
        if (!isEditMode && !formData.url.trim()) {
            formError = "URL is required.";
            return null;
        }
        if (urlTouched && !formData.url.trim()) {
            formError = "URL cannot be empty.";
            return null;
        }
        if (!formData.templateBody.trim()) {
            formError = "Template body is required.";
            return null;
        }
        if (headersTouched) {
            const invalid = headers.some((header) => header.key.trim() && !header.value.trim());
            if (invalid) {
                formError = "Header values are required when modifying headers.";
                return null;
            }
        }

        const basePayload = {
            name: formData.name.trim(),
            description: formData.description.trim(),
            type: formData.type,
            content_type: formData.contentType.trim() || "application/json",
            template: {
                format: formData.templateFormat.trim() || "json",
                body: formData.templateBody,
            },
            is_active: formData.isActive,
        };

        if (isEditMode) {
            const payload: WebhookUpdateReq = { ...basePayload };
            if (urlTouched) {
                payload.url = formData.url.trim();
            }
            if (headersTouched) {
                payload.headers = headers
                    .filter((header) => header.key.trim())
                    .map((header) => ({ key: header.key.trim(), value: header.value }));
            }
            return payload;
        }

        return {
            ...basePayload,
            url: formData.url.trim(),
            headers: headers
                .filter((header) => header.key.trim())
                .map((header) => ({ key: header.key.trim(), value: header.value })),
        } as WebhookCreateReq;
    }

    async function handleSubmit() {
        const payload = buildPayload();
        if (!payload) {
            return;
        }

        try {
            loading = true;
            await onSave(payload);
        } catch (err) {
            handleInlineError(
                err,
                isEditMode
                    ? "Unable to Update Webhook"
                    : "Unable to Create Webhook",
            );
        } finally {
            loading = false;
        }
    }

    async function handleTest() {
        if (!onTest) return;
        try {
            testing = true;
            await onTest();
        } catch (err) {
            handleInlineError(err, "Unable to Test Webhook");
        } finally {
            testing = false;
        }
    }

    function handleClose() {
        onClose();
    }

    function handleContentTypeInput(event: Event) {
        contentTypeTouched = true;
        formData.contentType = (event.target as HTMLInputElement).value;
    }

    function handleTemplateInput(event: Event) {
        templateTouched = true;
        formData.templateBody = (event.target as HTMLTextAreaElement).value;
    }

    // Close on Escape key
    function handleKeydown(event: KeyboardEvent) {
        if (event.key === "Escape") {
            handleClose();
        }
    }
</script>

<svelte:window on:keydown={handleKeydown} />

<div
    class="fixed inset-0 z-50 flex items-center justify-center bg-gray-900/60"
    onclick={handleClose}
    role="dialog"
    aria-modal="true"
>
    <div
        class="bg-white rounded-lg shadow-lg w-full max-w-5xl p-6 max-h-[90vh] flex flex-col"
        onclick={(e) => e.stopPropagation()}
    >
        <div class="flex items-center justify-between mb-6 flex-shrink-0">
            <div>
                <h3 class="font-bold text-lg text-gray-900">
                    {isEditMode ? "Edit Webhook" : "Add Webhook"}
                </h3>
                <p class="text-sm text-gray-500">
                    Configure a webhook destination for flow notifications.
                </p>
            </div>
            {#if isEditMode}
                <button
                    type="button"
                    onclick={handleTest}
                    disabled={testing || loading}
                    class="inline-flex items-center px-4 py-2 text-sm font-medium text-primary-700 bg-primary-50 rounded-lg hover:bg-primary-100 disabled:opacity-50 cursor-pointer"
                >
                    {testing ? "Testing..." : "Send Test"}
                </button>
            {/if}
        </div>

        <form
            onsubmit={(e) => {
                e.preventDefault();
                handleSubmit();
            }}
            class="flex flex-col flex-1 min-h-0"
        >
            <div class="grid grid-cols-3 gap-6 flex-1 overflow-y-auto pr-2">
                <div class="col-span-2 space-y-5">
                <div class="grid grid-cols-2 gap-4">
                    <div>
                        <label class="block text-sm font-medium text-gray-700 mb-1">
                            Name *
                        </label>
                        <input
                            type="text"
                            bind:value={formData.name}
                            class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent text-sm"
                            placeholder="Slack Prod Alerts"
                            required
                            disabled={loading}
                            use:autofocus
                        />
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-700 mb-1">
                            Type *
                        </label>
                        <select
                            bind:value={formData.type}
                            onchange={handleTypeChange}
                            class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent text-sm"
                            disabled={loading}
                        >
                            <option value="generic">Generic</option>
                            <option value="slack">Slack</option>
                            <option value="teams">Teams</option>
                        </select>
                    </div>
                </div>

                <div>
                    <label class="block text-sm font-medium text-gray-700 mb-1">
                        Description
                    </label>
                    <input
                        type="text"
                        bind:value={formData.description}
                        class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent text-sm"
                        placeholder="Optional description"
                        disabled={loading}
                    />
                </div>

                <div class="grid grid-cols-2 gap-4">
                    <div>
                        <label class="block text-sm font-medium text-gray-700 mb-1">
                            URL {isEditMode ? "(leave blank to keep)" : "*"}
                        </label>
                        <input
                            type="url"
                            value={formData.url}
                            oninput={handleUrlInput}
                            class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent text-sm"
                            placeholder={urlMasked || "https://hooks.example.com/..."}
                            disabled={loading}
                        />
                        <p class="text-xs text-gray-500 mt-1">
                            HTTPS only. URL is stored securely.
                        </p>
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-700 mb-1">
                            Content-Type
                        </label>
                        <input
                            type="text"
                            value={formData.contentType}
                            oninput={handleContentTypeInput}
                            class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent text-sm"
                            placeholder="application/json"
                            disabled={loading}
                        />
                    </div>
                </div>

                <div>
                    <label class="block text-sm font-medium text-gray-700 mb-2">
                        Headers
                    </label>
                    <div class="space-y-2">
                        {#each headers as header, index (index)}
                            <div class="grid grid-cols-2 gap-2">
                                <input
                                    type="text"
                                    class="px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-primary-500 text-sm"
                                    placeholder="Header key"
                                    value={header.key}
                                    oninput={(e) => updateHeader(index, "key", (e.target as HTMLInputElement).value)}
                                    disabled={loading}
                                />
                                <div class="flex items-center gap-2">
                                    <input
                                        type="text"
                                        class="flex-1 px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-primary-500 text-sm"
                                        placeholder={header.masked ? "********" : "Header value"}
                                        value={header.value}
                                        oninput={(e) => updateHeader(index, "value", (e.target as HTMLInputElement).value)}
                                        disabled={loading}
                                    />
                                    <button
                                        type="button"
                                        onclick={() => removeHeader(index)}
                                        class="text-danger-600 hover:text-danger-700 text-sm font-medium cursor-pointer"
                                    >
                                        Remove
                                    </button>
                                </div>
                            </div>
                        {/each}
                    </div>
                    <div class="flex items-center justify-between mt-2">
                        <button
                            type="button"
                            onclick={addHeader}
                            class="text-sm text-primary-600 hover:text-primary-700 font-medium cursor-pointer"
                        >
                            + Add header
                        </button>
                        {#if isEditMode}
                            <span class="text-xs text-gray-500">
                                Updating headers requires re-entering values.
                            </span>
                        {/if}
                    </div>
                </div>

                <div class="grid grid-cols-2 gap-4">
                    <div>
                        <label class="block text-sm font-medium text-gray-700 mb-1">
                            Template format
                        </label>
                        <input
                            type="text"
                            bind:value={formData.templateFormat}
                            class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-primary-500 text-sm"
                            disabled={loading}
                        />
                    </div>
                    <div class="flex items-center gap-3">
                        <input
                            type="checkbox"
                            bind:checked={formData.isActive}
                            class="h-4 w-4 text-primary-600 focus:ring-primary-500 border-gray-300 rounded"
                            disabled={loading}
                        />
                        <span class="text-sm text-gray-700">Active</span>
                    </div>
                </div>

                <div>
                    <label class="block text-sm font-medium text-gray-700 mb-2">
                        Template body *
                    </label>
                    <textarea
                        class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-primary-500 text-sm font-mono h-40"
                        value={formData.templateBody}
                        oninput={handleTemplateInput}
                        disabled={loading}
                    ></textarea>
                </div>

                <button
                    type="button"
                    onclick={() => (previewOpen = !previewOpen)}
                    class="text-sm text-primary-600 hover:text-primary-700 font-medium cursor-pointer"
                >
                    {previewOpen ? "Hide Preview" : "Preview Request"}
                </button>

                {#if previewOpen}
                    <div class="border border-gray-200 rounded-lg p-4 bg-gray-50">
                        <h4 class="text-sm font-semibold text-gray-700 mb-2">
                            Preview (sample data)
                        </h4>
                        <div class="text-xs text-gray-600 mb-2">
                            Headers:
                            <span class="ml-2 font-mono">Content-Type: {formData.contentType || "application/json"}</span>,
                            <span class="ml-2 font-mono">User-Agent: flowctl</span>,
                            <span class="ml-2 font-mono">X-Flowctl-Event: on_success</span>,
                            <span class="ml-2 font-mono">X-Flowctl-Delivery-Id: delivery-123</span>
                        </div>
                        <pre class="text-xs bg-white border border-gray-200 rounded-md p-3 overflow-auto">{renderPreview(formData.templateBody)}</pre>
                    </div>
                {/if}

                {#if formError}
                    <div class="text-sm text-danger-600">{formError}</div>
                {/if}

                </div>

                <aside class="col-span-1 space-y-4">
                    <div class="border border-gray-200 rounded-lg p-4">
                        <h4 class="text-sm font-semibold text-gray-700 mb-2">
                            Variables
                        </h4>
                        <div class="space-y-2 max-h-96 overflow-auto">
                            {#each variables as item}
                                <button
                                    type="button"
                                    onclick={() => insertVariable(item.token)}
                                    class="w-full text-left px-2 py-2 rounded-md hover:bg-gray-100 cursor-pointer"
                                >
                                    <div class="text-xs font-mono text-primary-600">{item.label}</div>
                                    <div class="text-xs text-gray-500">{item.description}</div>
                                </button>
                            {/each}
                        </div>
                    </div>
                </aside>
            </div>
            <div class="flex justify-end gap-2 pt-4 mt-4 border-t border-gray-200 flex-shrink-0 bg-white sticky bottom-0">
                <button
                    type="button"
                    class="inline-flex items-center px-5 py-2.5 text-sm font-medium text-gray-700 bg-gray-100 rounded-lg hover:bg-gray-200 disabled:opacity-50 cursor-pointer"
                    onclick={handleClose}
                    disabled={loading}
                >
                    Cancel
                </button>
                <button
                    type="submit"
                    class="inline-flex items-center px-5 py-2.5 text-sm font-medium text-white bg-primary-500 rounded-lg hover:bg-primary-600 focus:ring-4 focus:outline-none focus:ring-primary-300 disabled:opacity-50 cursor-pointer"
                    disabled={loading}
                >
                    {#if loading}
                        <svg
                            class="animate-spin -ml-1 mr-2 h-4 w-4 text-white"
                            xmlns="http://www.w3.org/2000/svg"
                            fill="none"
                            viewBox="0 0 24 24"
                        >
                            <circle
                                class="opacity-25"
                                cx="12"
                                cy="12"
                                r="10"
                                stroke="currentColor"
                                stroke-width="4"
                            ></circle>
                            <path
                                class="opacity-75"
                                fill="currentColor"
                                d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                            ></path>
                        </svg>
                    {/if}
                    {isEditMode ? "Update" : "Create"}
                </button>
            </div>
        </form>
    </div>
</div>
