<script lang="ts">
    import { createSlug, isValidCronExpression } from "$lib/utils";
    import { getTimezones } from "$lib/utils/timezone";
    import { get } from "svelte/store";
    import { appInfo } from "$lib/stores/auth";
    import type { Schedule } from "$lib/types";
    import FlowGroupSelector from "./FlowGroupSelector.svelte";

    let {
        metadata = $bindable(),
        namespace,
        inputs = [],
        updatemode = false,
        disabled = false,
    }: {
        metadata: {
            id: string;
            name: string;
            description: string;
            prefix: string;
            schedules: Schedule[];
            namespace: string;
            allow_overlap: boolean;
            user_schedulable: boolean;
            execution_mode?: string;
            max_parallel?: number;
        };
        namespace: string;
        inputs?: any[];
        updatemode?: boolean;
        disabled?: boolean;
    } = $props();

    let hasFileInputs = $derived(
        inputs.some((input) => input.type === "file"),
    );

    let hasMissingDefaults = $derived(
        inputs.some((input) => input.type !== "file" && (!input.default || input.default.trim() === "")),
    );

    let isSchedulable = $derived(!hasFileInputs && !hasMissingDefaults);

    function updateName(value: string) {
        if (updatemode) return;
        metadata.name = value;
        metadata.id = createSlug(value);
    }

    function updateDescription(value: string) {
        metadata.description = value;
    }

    function setDAGMode(enabled: boolean) {
        metadata.execution_mode = enabled ? "dag" : "sequential";
    }

    const timezones = getTimezones();

    function addSchedule() {
        if (!metadata.schedules) {
            metadata.schedules = [];
        }
        metadata.schedules.push({ name: "", cron: "", timezone: get(appInfo)?.default_timezone ?? "UTC" });
    }

    function removeSchedule(index: number) {
        metadata.schedules.splice(index, 1);
    }

    function updateScheduleCron(index: number, value: string) {
        if (!metadata.schedules) {
            metadata.schedules = [];
        }
        metadata.schedules[index].cron = value;
    }

    function updateScheduleName(index: number, value: string) {
        if (!metadata.schedules) {
            metadata.schedules = [];
        }
        metadata.schedules[index].name = value;
    }

    function updateScheduleTimezone(index: number, value: string) {
        if (!metadata.schedules) {
            metadata.schedules = [];
        }
        metadata.schedules[index].timezone = value;
    }

    let scheduleValidations = $derived(
        metadata.schedules?.map((schedule) => ({
            schedule,
            isValid:
                schedule.cron === "" || isValidCronExpression(schedule.cron),
        })) || [],
    );
</script>

<!-- Flow Metadata Section -->
<div>
    <div class="vstack gap-4">
        <div data-field>
            <label for="flow-name">Flow Name *</label>
            <input
                type="text"
                id="flow-name"
                value={metadata.name}
                oninput={(e) => updateName(e.currentTarget.value)}
                placeholder="My Flow Name"
                disabled={updatemode}
            />
        </div>
        <div data-field>
            <label for="flow-description">Description</label>
            <textarea
                id="flow-description"
                value={metadata.description}
                oninput={(e) => updateDescription(e.currentTarget.value)}
                placeholder="Describe what this flow does..."
                rows="3"
            ></textarea>
        </div>
        <FlowGroupSelector {namespace} bind:value={metadata.prefix} />
    </div>

    <!-- Execution Subsection -->
    <div class="scheduling-section mt-6">
        <h3>Execution</h3>

        <div class="mb-4">
            <label class="hstack gap-2 checkbox-label">
                <input
                    type="checkbox"
                    checked={metadata.execution_mode === "dag"}
                    onchange={(e) =>
                        setDAGMode(e.currentTarget.checked)}
                />
                <span>Run Actions as a Dependency Graph</span>
            </label>
            <p class="text-lighter hint indent">
                Actions declare which other actions they depend on and run as
                soon as those finish, so independent actions run at the same
                time. When off, actions run one after another in listed order.
            </p>
        </div>

        {#if metadata.execution_mode === "dag"}
            <div data-field class="mb-4 indent">
                <label for="max-parallel">Maximum Parallel Actions</label>
                <input
                    type="number"
                    id="max-parallel"
                    min="0"
                    max="64"
                    bind:value={metadata.max_parallel}
                />
                <p class="text-lighter hint">
                    Most actions to run at the same time. Set to 0 for no limit.
                </p>
            </div>
        {/if}
    </div>

    <!-- Scheduling Subsection -->
    <div class="scheduling-section mt-6">
        <h3>Scheduling</h3>

        <div class="mb-4">
            <label class="hstack gap-2 checkbox-label">
                <input
                    type="checkbox"
                    bind:checked={metadata.allow_overlap}
                />
                <span>Allow Overlapping Executions</span>
            </label>
            <p class="text-lighter hint indent">
                If enabled, new executions can start even if a previous
                execution is still running / waiting for approval.
            </p>
        </div>

        {#if !isSchedulable}
            <div class="warning-box mb-4">
                <div class="hstack gap-2 items-start">
                    <svg class="warning-icon" viewBox="0 0 20 20" fill="currentColor">
                        <path fill-rule="evenodd" d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z" clip-rule="evenodd" />
                    </svg>
                    <div>
                        <h4 class="warning-title">Warning</h4>
                        <p class="warning-text">
                            This flow has inputs without default values. Scheduled executions may produce unwanted results if required inputs are not provided.
                        </p>
                    </div>
                </div>
            </div>
        {/if}

        <div class="mb-4">
            <label class="hstack gap-2 checkbox-label">
                <input
                    type="checkbox"
                    bind:checked={metadata.user_schedulable}
                />
                <span>Allow User Schedules</span>
            </label>
            <p class="text-lighter hint indent">
                Users can create their own cron schedules for this flow
            </p>
        </div>

        <div>
            <div class="hstack mb-2 justify-between">
                <label>
                    Cron Schedules
                    <span class="text-lighter label-hint">(optional)</span>
                </label>
                {#if !disabled}
                    <button
                        type="button"
                        data-variant="secondary"
                        class="add-schedule-btn"
                        onclick={addSchedule}
                    >
                        + Add Schedule
                    </button>
                {/if}
            </div>

            <div class="vstack gap-4">
                {#each metadata.schedules || [] as schedule, index}
                    {@const validation = scheduleValidations[index]}
                    <div class="schedule-card p-4">
                        <div class="hstack gap-2 items-start">
                            <div class="schedule-grid">
                                <div data-field>
                                    <label for="schedule-name-{index}">Name</label>
                                    <input
                                        id="schedule-name-{index}"
                                        type="text"
                                        value={schedule.name || ''}
                                        oninput={(e) => updateScheduleName(index, e.currentTarget.value)}
                                        maxlength="150"
                                        placeholder="Cron name"
                                    />
                                </div>
                                <div data-field>
                                    <label>Cron Expression</label>
                                    <input
                                        type="text"
                                        value={schedule.cron}
                                        oninput={(e) => updateScheduleCron(index, e.currentTarget.value)}
                                        aria-invalid={schedule.cron && !validation?.isValid ? 'true' : undefined}
                                        placeholder="0 2 * * *"
                                    />
                                    {#if schedule.cron && !validation?.isValid}
                                        <p style="color: var(--danger); margin-top: 0.25rem" class="text-sm">
                                            Invalid cron expression. Use
                                            format: minute hour day month
                                            weekday
                                        </p>
                                    {/if}
                                </div>
                                <div data-field>
                                    <label>Timezone</label>
                                    <input
                                        type="text"
                                        list="timezone-list-{index}"
                                        value={schedule.timezone}
                                        oninput={(e) => updateScheduleTimezone(index, e.currentTarget.value)}
                                        placeholder="Search or select timezone..."
                                    />
                                    <datalist id="timezone-list-{index}">
                                        {#each timezones as tz}
                                            <option value={tz.tzCode}>{tz.label}</option>
                                        {/each}
                                    </datalist>
                                </div>
                            </div>
                            {#if !disabled}
                                <div class="remove-schedule-field">
                                    <span class="label-spacer" aria-hidden="true">&nbsp;</span>
                                    <button
                                        type="button"
                                        data-variant="danger"
                                        class="ghost icon remove-schedule-btn"
                                        title="Remove schedule"
                                        aria-label="Remove schedule"
                                        onclick={() => removeSchedule(index)}
                                    >
                                        <svg class="icon-sm" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                                        </svg>
                                    </button>
                                </div>
                            {/if}
                        </div>
                    </div>
                {/each}

                {#if !metadata.schedules || metadata.schedules.length === 0}
                    <div class="empty-state vstack">
                        <svg class="empty-icon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                        </svg>
                        <p>No schedules defined</p>
                        {#if !disabled}
                            <button
                                type="button"
                                data-variant="secondary"
                                onclick={addSchedule}
                            >
                                Add your first schedule
                            </button>
                        {/if}
                    </div>
                {/if}
            </div>

            <p class="text-lighter hint mt-2">
                Use cron expression format. You can add multiple schedules
                for different execution times.
                <br />
                Examples:
                <code>0 2 * * *</code> (daily 2AM),
                <code>0 */6 * * *</code> (every 6 hours)
            </p>
        </div>

        <div data-field class="mt-4">
            <label for="max-retries">Maximum Retries</label>
            <input
                type="number"
                id="max-retries"
                min="0"
                max="10"
                bind:value={metadata.max_retries}
            />
            <p class="text-lighter hint">
                Number of times to automatically retry the flow on failure.
                Uses exponential backoff (15s initial delay, up to 5min).
                Set to 0 to disable auto-retry.
            </p>
        </div>
    </div>
</div>

<style>
    .scheduling-section {
        padding-top: 1.5rem;
        border-top: 1px solid var(--border);
    }
    .scheduling-section h3 {
        font-size: 1.125rem;
        font-weight: 600;
        color: var(--foreground);
        margin-bottom: 1rem;
    }
    .checkbox-label {
        cursor: pointer;
    }
    .checkbox-label span {
        font-size: 0.875rem;
        font-weight: 500;
        color: var(--foreground);
    }
    .hint {
        font-size: 0.75rem;
        margin-top: 0.25rem;
    }
    .indent {
        margin-left: 1.5rem;
    }
    .label-hint {
        font-weight: normal;
        font-size: 0.875rem;
    }
    .warning-box {
        background: var(--warning);
        border: 1px solid var(--warning);
        border-radius: 0.375rem;
        padding: 1rem;
        opacity: 0.9;
    }
    .warning-icon {
        width: 1.25rem;
        height: 1.25rem;
        color: var(--background);
        flex-shrink: 0;
    }
    .warning-title {
        font-size: 0.875rem;
        font-weight: 500;
        color: var(--background);
    }
    .warning-text {
        font-size: 0.875rem;
        color: var(--background);
        margin-top: 0.5rem;
    }
    .schedule-card {
        border: 1px solid var(--border);
        border-radius: 0.375rem;
    }
    .schedule-grid {
        flex: 1;
        display: grid;
        grid-template-columns: repeat(3, minmax(0, 1fr));
        gap: 0.75rem;
    }
    @media (max-width: 768px) {
        .schedule-grid {
            grid-template-columns: 1fr;
        }
    }
    .remove-schedule-field {
        display: flex;
        flex-direction: column;
        flex-shrink: 0;
    }
    .label-spacer {
        font-size: var(--text-7);
        line-height: var(--leading-normal);
    }
    .remove-schedule-btn {
        /* Same height as an input: line box + padding + borders. */
        height: calc(var(--text-7) * var(--leading-normal) + var(--space-2) * 2 + 2px);
        margin-block-start: var(--space-1);
    }
    .icon-sm {
        width: 1.25rem;
        height: 1.25rem;
    }
    .add-schedule-btn {
        font-size: 0.75rem;
        padding: 0.125rem 0.5rem;
        border: none;
        background: none;
    }
    .error {
        border-color: var(--danger) !important;
    }
    .field-error {
        font-size: 0.75rem;
        color: var(--danger);
        margin-top: 0.25rem;
    }
    .empty-state {
        text-align: center;
        padding: 1.5rem 1rem;
        border: 2px dashed var(--border);
        border-radius: 0.375rem;
        color: var(--muted-foreground);
        align-items: center;
        gap: 0.5rem;
    }
    .empty-icon {
        width: 2rem;
        height: 2rem;
        color: var(--muted-foreground);
    }
    .p-4 { padding: 1rem; }
    .mb-2 { margin-bottom: 0.5rem; }
    .mb-4 { margin-bottom: 1rem; }
    .mt-2 { margin-top: 0.5rem; }
    .mt-6 { margin-top: 2rem; }
</style>
