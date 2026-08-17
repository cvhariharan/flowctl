<script lang="ts">
    import MultiReceiverSelector from "$lib/components/shared/MultiReceiverSelector.svelte";
    import OatSelect from "$lib/components/shared/OatSelect.svelte";
    import type { Notify } from "$lib/types";

    let {
        notifications = $bindable(),
        availableMessengers,
        messengerConfigs,
        disabled = false,
    }: {
        notifications: Notify[];
        availableMessengers: string[];
        messengerConfigs: Record<string, any>;
        disabled?: boolean;
    } = $props();

    function addNotification() {
        notifications.push({ channel: "", events: [], config: {} });
    }

    function removeNotification(index: number) {
        notifications.splice(index, 1);
    }

    const eventOptions = [
        { value: "on_success", label: "On Success" },
        { value: "on_failure", label: "On Failure" },
        { value: "on_waiting", label: "On Waiting" },
        { value: "on_cancelled", label: "On Cancelled" },
    ];

    function onChannelChange(notification: any) {
        const schema = messengerConfigs[notification.channel];
        if (!schema?.properties) {
            notification.config = {};
            return;
        }
        const config: Record<string, any> = {};
        for (const [key, property] of Object.entries(schema.properties) as [string, any][]) {
            if (notification.config && notification.config[key] !== undefined) {
                config[key] = notification.config[key];
            } else if (property.type === "array" || property.widget === "userselector") {
                config[key] = [];
            } else {
                config[key] = "";
            }
        }
        notification.config = config;
    }

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
</script>

<!-- Flow Notifications Section -->
<div>
    <div class="hstack mb-4 justify-between">
        <div>
            <h3>Flow Notifications</h3>
            <p class="text-light">Configure notifications for flow execution events</p>
        </div>
        {#if !disabled}
            <button onclick={addNotification}>
                + Add Notification
            </button>
        {/if}
    </div>

    <div class="vstack gap-4">
        {#each notifications as notification, index (index)}
            <div class="notification-card">
                {#if !disabled}
                    <button
                        type="button"
                        data-variant="danger"
                        class="remove-btn"
                        onclick={() => removeNotification(index)}
                    >
                        <svg class="icon-sm" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                        </svg>
                    </button>
                {/if}

                <div class="vstack gap-4">
                    <!-- Channel -->
                    <div data-field>
                        <label>Channel *</label>
                        <OatSelect
                            bind:value={notification.channel}
                            options={availableMessengers.map((m: string) => ({
                                value: m,
                                label: m.charAt(0).toUpperCase() + m.slice(1)
                            }))}
                            placeholder={availableMessengers.length === 0
                                ? "No messengers available"
                                : "Select channel"}
                            onchange={() => onChannelChange(notification)}
                            required
                        />
                    </div>

                    <!-- Events -->
                    <div data-field>
                        <label>Events *</label>
                        <div class="vstack gap-1">
                            {#each eventOptions as event}
                                <label class="hstack gap-2 checkbox-label">
                                    <input
                                        type="checkbox"
                                        checked={notification.events?.includes(event.value)}
                                        onchange={() => toggleEvent(notification, event.value)}
                                    />
                                    <span>{event.label}</span>
                                </label>
                            {/each}
                        </div>
                    </div>

                    <!-- Dynamic messenger config fields -->
                    {#if notification.channel && messengerConfigs[notification.channel]?.properties}
                        {@const schema = messengerConfigs[notification.channel]}
                        {#each Object.entries(schema.properties) as [key, property]}
                            {@const isRequired = schema.required?.includes(key)}
                            {@const label = property.title || key}
                            {@const description = property.description || ""}

                            <div data-field>
                                {#if property.widget === "userselector"}
                                    <label>
                                        {label}
                                        {#if isRequired}<span class="req">*</span>{/if}
                                    </label>
                                    <MultiReceiverSelector
                                        bind:selectedReceivers={notification.config[key]}
                                    />
                                {:else}
                                    <label>
                                        {label}
                                        {#if isRequired}<span class="req">*</span>{/if}
                                    </label>
                                    <input
                                        type="text"
                                        bind:value={notification.config[key]}
                                        placeholder={property.placeholder || ""}
                                    />
                                {/if}
                                {#if description}
                                    <span data-hint>{description}</span>
                                {/if}
                            </div>
                        {/each}
                    {/if}
                </div>
            </div>
        {/each}

        {#if notifications.length === 0}
            <div class="empty-state vstack">
                <svg class="empty-icon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9" />
                </svg>
                <p>No notifications configured yet</p>
                {#if !disabled}
                    <button data-variant="secondary" onclick={addNotification}>
                        Add your first notification
                    </button>
                {/if}
            </div>
        {/if}
    </div>
</div>

<style>
    .notification-card {
        border: 1px solid var(--border);
        border-radius: 0.5rem;
        padding: 1rem;
        position: relative;
    }
    .remove-btn {
        position: absolute;
        top: 1rem;
        right: 1rem;
        padding: 0.25rem;
        border: none;
        background: none;
    }
    .icon-sm {
        width: 1.25rem;
        height: 1.25rem;
    }
    .checkbox-label {
        cursor: pointer;
        font-size: 0.875rem;
    }
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
