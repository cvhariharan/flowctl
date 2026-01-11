<script lang="ts">
    import { autofocus } from "$lib/utils/autofocus";
    import { handleInlineError } from "$lib/utils/errorHandling";
    import type { WebhookListItem } from "$lib/types";

    interface DuplicatePayload {
        name: string;
        description?: string;
    }

    interface Props {
        webhook: WebhookListItem;
        onConfirm: (payload: DuplicatePayload) => Promise<void> | void;
        onClose: () => void;
    }

    let { webhook, onConfirm, onClose }: Props = $props();

    let formData = $state({
        name: "",
        description: "",
    });

    let loading = $state(false);
    let formError = $state("");

    $effect(() => {
        if (webhook) {
            formData = {
                name: `${webhook.name} Copy`,
                description: webhook.description || "",
            };
            formError = "";
        }
    });

    async function handleSubmit() {
        formError = "";
        if (!formData.name.trim()) {
            formError = "Name is required.";
            return;
        }

        try {
            loading = true;
            await onConfirm({
                name: formData.name.trim(),
                description: formData.description.trim() || undefined,
            });
        } catch (err) {
            handleInlineError(err, "Unable to Duplicate Webhook");
        } finally {
            loading = false;
        }
    }

    function handleClose() {
        onClose();
    }

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
        class="bg-white rounded-lg shadow-lg w-full max-w-xl p-6"
        onclick={(e) => e.stopPropagation()}
    >
        <h3 class="font-bold text-lg text-gray-900 mb-2">Duplicate Webhook</h3>
        <p class="text-sm text-gray-500 mb-6">
            Create a new webhook by copying settings from
            <span class="font-medium text-gray-700">{webhook.name}</span>.
        </p>

        <form
            onsubmit={(e) => {
                e.preventDefault();
                handleSubmit();
            }}
            class="space-y-4"
        >
            <div>
                <label class="block text-sm font-medium text-gray-700 mb-1">
                    Name *
                </label>
                <input
                    type="text"
                    bind:value={formData.name}
                    class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent text-sm"
                    placeholder="My Webhook Copy"
                    required
                    disabled={loading}
                    use:autofocus
                />
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

            {#if formError}
                <div class="text-sm text-danger-600">{formError}</div>
            {/if}

            <div class="flex justify-end gap-2 pt-2">
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
                    Duplicate
                </button>
            </div>
        </form>
    </div>
</div>
