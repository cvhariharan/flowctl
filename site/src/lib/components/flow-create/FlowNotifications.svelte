<script lang="ts">
    import { onMount } from "svelte";
    import MultiReceiverSelector from "$lib/components/shared/MultiReceiverSelector.svelte";
    import { apiClient } from "$lib/apiClient";
    import { handleInlineError } from "$lib/utils/errorHandling";
    import type { WebhookListItem } from "$lib/types";

    let {
        notifications = $bindable(),
        addNotification,
        namespace,
    }: {
        notifications: any[];
        addNotification: () => void;
        namespace: string;
    } = $props();

    let messengers: string[] = $state([]);
    let loadingMessengers = $state(true);
    let webhooks = $state<WebhookListItem[]>([]);
    let loadingWebhooks = $state(true);

    onMount(async () => {
        try {
            const response = await fetch("/api/v1/messengers");
            if (response.ok) {
                const data = await response.json();
                messengers = data.messengers || [];
            } else {
                console.error("Failed to fetch messengers");
                messengers = [];
            }
        } catch (error) {
            console.error("Error fetching messengers:", error);
            messengers = [];
        } finally {
            loadingMessengers = false;
        }

        if (!namespace) {
            loadingWebhooks = false;
            return;
        }

        try {
            const response = await apiClient.namespaceWebhooks.list(namespace);
            webhooks = response.webhooks || [];
        } catch (error) {
            handleInlineError(error, "Unable to Load Webhooks");
            webhooks = [];
        } finally {
            loadingWebhooks = false;
        }
    });

    function removeNotification(index: number) {
        notifications.splice(index, 1);
    }

    const eventOptions = [
        { value: "on_success", label: "On Success" },
        { value: "on_failure", label: "On Failure" },
        { value: "on_waiting", label: "On Waiting" },
        { value: "on_cancelled", label: "On Cancelled" },
    ];

    function toggleEvent(notification: any, eventValue: string) {
        if (!notification.events) {
            notification.events = [];
        }
        const index = notification.events.indexOf(eventValue);
        if (index > -1) {
            notification.events.splice(index, 1);
        } else {
            notification.events.push(eventValue);
        }
    }

    function handleChannelChange(notification: any, value: string) {
        notification.channel = value;
        if (value === "webhook") {
            notification.receivers = [];
            notification.webhook_names = notification.webhook_names || [];
            notification.template_override =
                notification.template_override || { body: "" };
        } else {
            notification.webhook_names = [];
            notification.template_override = undefined;
            notification.receivers = notification.receivers || [];
        }
    }

    function toggleWebhook(notification: any, name: string) {
        if (!notification.webhook_names) {
            notification.webhook_names = [];
        }
        const index = notification.webhook_names.indexOf(name);
        if (index > -1) {
            notification.webhook_names.splice(index, 1);
        } else {
            notification.webhook_names.push(name);
        }
    }

    $effect(() => {
        for (const notification of notifications) {
            if (notification.channel === "webhook" && !notification.template_override) {
                notification.template_override = { body: "" };
            }
        }
    });
</script>

<!-- Flow Notifications Section -->
<div>
    <div class="flex items-center justify-between mb-6">
        <div>
            <h3 class="text-base font-medium text-gray-900">
                Flow Notifications
            </h3>
            <p class="mt-1 text-sm text-gray-500">
                Configure notifications for flow execution events
            </p>
        </div>
        <button
            onclick={addNotification}
            class="px-4 py-2 text-sm font-medium bg-primary-500 text-white rounded-md hover:bg-primary-600 focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2 cursor-pointer"
        >
            + Add Notification
        </button>
    </div>

    <div class="space-y-4">
        {#each notifications as notification, index (index)}
            <div class="border border-gray-200 rounded-lg p-4 relative">
                <button
                    onclick={() => removeNotification(index)}
                    class="absolute top-4 right-4 text-gray-400 hover:text-danger-600 cursor-pointer"
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

                <div class="space-y-4">
                    <!-- Channel -->
                    <div>
                        <label
                            class="block text-sm font-medium text-gray-700 mb-1"
                            >Channel *</label
                        >
                        <select
                            bind:value={notification.channel}
                            onchange={(e) => handleChannelChange(notification, (e.target as HTMLSelectElement).value)}
                            disabled={loadingMessengers}
                            required
                            class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent text-sm disabled:bg-gray-100 disabled:cursor-not-allowed"
                        >
                            <option value="">
                                {loadingMessengers
                                    ? "Loading..."
                                    : messengers.length === 0
                                      ? "No messengers available"
                                      : "Select channel"}
                            </option>
                            {#each messengers as messenger}
                                <option value={messenger}>
                                    {messenger.charAt(0).toUpperCase() +
                                        messenger.slice(1)}
                                </option>
                            {/each}
                        </select>
                    </div>

                    <!-- Events -->
                    <div>
                        <label
                            class="block text-sm font-medium text-gray-700 mb-2"
                            >Events *</label
                        >
                        <div class="space-y-2">
                            {#each eventOptions as event}
                                <label class="flex items-center cursor-pointer">
                                    <input
                                        type="checkbox"
                                        checked={notification.events?.includes(
                                            event.value,
                                        )}
                                        onchange={() =>
                                            toggleEvent(
                                                notification,
                                                event.value,
                                            )}
                                        class="h-4 w-4 text-primary-600 focus:ring-primary-500 border-gray-300 rounded"
                                    />
                                    <span class="ml-2 text-sm text-gray-700"
                                        >{event.label}</span
                                    >
                                </label>
                            {/each}
                        </div>
                    </div>

                    {#if notification.channel === "webhook"}
                        <div>
                            <div class="flex items-center justify-between mb-2">
                                <label class="block text-sm font-medium text-gray-700">
                                    Webhooks *
                                </label>
                                <a
                                    href="/view/{namespace}/webhooks"
                                    target="_blank"
                                    rel="noopener noreferrer"
                                    class="text-xs text-primary-600 hover:text-primary-700"
                                >
                                    Manage webhooks
                                </a>
                            </div>
                            {#if loadingWebhooks}
                                <div class="text-sm text-gray-500">Loading webhooks...</div>
                            {:else if webhooks.length === 0}
                                <div class="text-sm text-gray-500">
                                    No webhooks available yet.
                                </div>
                            {:else}
                                <div class="space-y-2 border border-gray-200 rounded-md p-3 bg-gray-50">
                                    {#each webhooks as webhook}
                                        <label class="flex items-center gap-2 text-sm text-gray-700">
                                            <input
                                                type="checkbox"
                                                checked={notification.webhook_names?.includes(webhook.name)}
                                                onchange={() => toggleWebhook(notification, webhook.name)}
                                                class="h-4 w-4 text-primary-600 focus:ring-primary-500 border-gray-300 rounded"
                                            />
                                            <span>{webhook.name}</span>
                                            {#if !webhook.is_active}
                                                <span class="text-xs text-warning-600">(inactive)</span>
                                            {/if}
                                        </label>
                                    {/each}
                                </div>
                            {/if}
                        </div>

                        <div>
                            <label class="block text-sm font-medium text-gray-700 mb-2">
                                Template Override (optional)
                            </label>
                            <textarea
                                class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-primary-500 text-sm font-mono h-24"
                                bind:value={notification.template_override.body}
                                placeholder="Leave blank to use the webhook default template"
                            ></textarea>
                            <p class="text-xs text-gray-500 mt-1">
                                Available variables: flow, execution, namespace, app.
                            </p>
                        </div>
                    {:else}
                        <div>
                            <label
                                class="block text-sm font-medium text-gray-700 mb-2"
                                >Receivers *</label
                            >
                            <MultiReceiverSelector
                                bind:selectedReceivers={notification.receivers}
                            />
                        </div>
                    {/if}
                </div>
            </div>
        {/each}

        {#if notifications.length === 0}
            <div class="text-center py-8 text-gray-500">
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
                        d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9"
                    />
                </svg>
                <p>No notifications configured yet</p>
                <button
                    onclick={addNotification}
                    class="mt-2 text-sm text-primary-600 hover:text-primary-700 font-medium cursor-pointer"
                >
                    Add your first notification
                </button>
            </div>
        {/if}
    </div>
</div>
