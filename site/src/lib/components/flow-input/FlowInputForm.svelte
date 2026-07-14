<script lang="ts">
  import { goto } from '$app/navigation';
  import type { FlowInput } from '$lib/types';
  import { ApiError, getCsrfToken } from '$lib/apiClient';
  import { handleInlineError, showSuccess } from '$lib/utils/errorHandling';
  import { getTimezones } from '$lib/utils/timezone';
  import { DateTime } from 'luxon';
  import { IconClock, IconPlayerPlay } from '@tabler/icons-svelte';
  import { get } from 'svelte/store';
  import { appInfo } from '$lib/stores/auth';
  import FlowInputFields from '$lib/components/shared/FlowInputFields.svelte';

  let {
    inputs,
    namespace,
    flowId,
    executionInput = null,
    optionsRequestId = undefined,
    onScheduled
  }: {
    inputs: FlowInput[],
    namespace: string,
    flowId: string,
    executionInput?: Record<string, any> | null,
    optionsRequestId?: string,
    onScheduled?: () => void
  } = $props();

  let loading = $state(false);
  let errors = $state<Record<string, string>>({});
  let scheduleEnabled = $state(false);
  let scheduledAt = $state('');
  let scheduledTimezone = $state(get(appInfo)?.default_timezone ?? Intl.DateTimeFormat().resolvedOptions().timeZone);

  const timezones = getTimezones();

  const hasInputValue = (name: string) =>
    executionInput !== null && Object.prototype.hasOwnProperty.call(executionInput, name);

  const formatInputValue = (input: FlowInput, value: any) => {
    if (value === null || value === undefined) return input.type === 'node' ? [] : '';

    if (input.type === 'checkbox') {
      if (typeof value === 'boolean') return value;
      if (typeof value === 'string') return value.toLowerCase() === 'true';
      return Boolean(value);
    }

    if (input.type === 'datetime') {
      return String(value).replace(/(:\d{2}(?:\.\d+)?)?(Z|[+-]\d{2}:\d{2})$/, '');
    }

    if (input.type === 'node') {
      if (Array.isArray(value)) return value;
      return typeof value === 'string' && value ? [value] : [];
    }

    return String(value);
  };

  const buildInitialValues = () => {
    const values: Record<string, any> = {};

    for (const input of inputs) {
      if (input.default !== undefined) {
        values[input.name] = formatInputValue(input, input.default);
      }
      if (hasInputValue(input.name)) {
        values[input.name] = formatInputValue(input, executionInput?.[input.name]);
      }
    }

    return values;
  };

  let initialValues = $state<Record<string, any>>(buildInitialValues());

  $effect(() => {
    initialValues = buildInitialValues();
  });

  const toRFC3339 = (localDateTime: string, timezone: string): string => {
    return DateTime.fromISO(localDateTime, { zone: timezone }).toISO() ?? localDateTime;
  };

  const submit = async (event: SubmitEvent) => {
    event.preventDefault();
    loading = true;
    errors = {};

    const form = event.target as HTMLFormElement;
    const formData = new FormData(form);

    let url = `/api/v1/${encodeURIComponent(namespace)}/trigger/${flowId}`;
    if (scheduleEnabled && scheduledAt) {
      const scheduledAtRFC3339 = toRFC3339(scheduledAt, scheduledTimezone);
      if (new Date(scheduledAtRFC3339) <= new Date()) {
        errors = { general: 'Scheduled time must be in the future' };
        loading = false;
        return;
      }
      url += `?scheduled_at=${encodeURIComponent(scheduledAtRFC3339)}`;
    }

    const headers: Record<string, string> = { 'X-CSRF-Token': getCsrfToken() };
    if (optionsRequestId) {
      headers['X-Options-Request-ID'] = optionsRequestId;
    }

    try {
      const response = await fetch(url, {
        method: 'POST',
        body: formData,
        credentials: 'include',
        headers,
      });

      if (!response.ok) {
        const errorData = await response.json().catch(() => ({}));

        if (errorData.details && errorData.details.field && errorData.details.error) {
          errors[errorData.details.field] = errorData.details.error;
        }
        else {
          const apiError = new ApiError(response.status, response.statusText, errorData);
          handleInlineError(apiError, 'Unable to Start Flow');
        }
      } else {
        const data = await response.json();
        if (data.scheduled_at) {
          const scheduledDate = new Date(data.scheduled_at);
          showSuccess('Flow Scheduled', `Flow will run at ${scheduledDate.toLocaleString()}`);
          scheduleEnabled = false;
          scheduledAt = '';
          onScheduled?.();
        } else {
          goto(`/view/${encodeURIComponent(namespace)}/results/${flowId}/${data.exec_id}`);
        }
      }
    } catch (error) {
      handleInlineError(error, 'Unable to Start Flow');
    } finally {
      loading = false;
    }
  };

</script>

<div class="card">
  <div class="card-header">
    <h2>Configuration Parameters</h2>
    <p class="text-light">Configure the inputs for this flow execution</p>
  </div>

  <form onsubmit={submit} class="form-body vstack gap-4">
    {#if errors.general}
      <div class="error-banner">
        <div>{errors.general}</div>
      </div>
    {/if}

    <FlowInputFields inputs={inputs} bind:values={initialValues} {errors} useFormData={true} {namespace} {flowId} />

    <!-- Schedule option -->
    <div class="schedule-section">
      <div class="hstack justify-between">
        <div class="hstack gap-2">
          <IconClock size={20} class="text-lighter" />
          <span class="schedule-label">Schedule for later</span>
        </div>
        <input
          type="checkbox"
          role="switch"
          checked={scheduleEnabled}
          onchange={() => scheduleEnabled = !scheduleEnabled}
        />
      </div>

      {#if scheduleEnabled}
        <div class="vstack gap-2 mt-4">
          <div data-field>
            <label for="scheduled_at">
              Run at
              <span class="required">*</span>
            </label>
            <input
              type="datetime-local"
              id="scheduled_at"
              bind:value={scheduledAt}
              required={scheduleEnabled}
            />
          </div>
          <div data-field>
            <label for="scheduled_timezone">Timezone</label>
            <input
              type="text"
              id="scheduled_timezone"
              list="timezone-list"
              bind:value={scheduledTimezone}
              placeholder="Search or select timezone..."
            />
            <datalist id="timezone-list">
              {#each timezones as tz}
                <option value={tz.tzCode}>{tz.label}</option>
              {/each}
            </datalist>
          </div>
          <p class="text-light hint">The flow will be queued and executed at the specified time</p>
        </div>
      {/if}
    </div>

    <div class="hstack gap-2 action-buttons">
      <button
        type="button"
        data-variant="secondary"
        onclick={() => window.history.back()}
        class="flex-1"
      >
        Cancel
      </button>
      <button
        type="submit"
        disabled={loading || (scheduleEnabled && !scheduledAt)}
        class="flex-1"
      >
        {#if loading}
          <span class="hstack gap-2 justify-center" aria-busy="true" data-spinner="small">
            {scheduleEnabled ? 'Scheduling...' : 'Starting Flow...'}
          </span>
        {:else if scheduleEnabled}
          <IconClock size={20} />
          Schedule
        {:else}
          <IconPlayerPlay size={20} />
          Run Now
        {/if}
      </button>
    </div>
  </form>
</div>

<style>
  .card {
    background: var(--card);
    border-radius: 0.5rem;
    border: 1px solid var(--border);
    overflow: hidden;
  }
  .card-header {
    padding: 1rem 1.5rem;
    border-bottom: 1px solid var(--border);
    background: var(--faint);
  }
  .card-header h2 {
    font-size: 1.125rem;
    font-weight: 600;
    color: var(--foreground);
  }
  .form-body {
    padding: 1.5rem;
  }
  .error-banner {
    padding: 0.75rem;
    border-radius: 0.375rem;
    background: color-mix(in srgb, var(--danger) 10%, transparent);
    border: 1px solid var(--danger);
    font-size: 0.875rem;
    color: var(--danger);
  }
  .schedule-section {
    padding-top: 1rem;
    border-top: 1px solid var(--border);
  }
  .schedule-label {
    font-size: 0.875rem;
    font-weight: 500;
    color: var(--foreground);
  }
  .required {
    color: var(--danger);
  }
  .hint {
    font-size: 0.875rem;
  }
  .action-buttons {
    padding-top: 1.5rem;
    border-top: 1px solid var(--border);
  }
  .flex-1 {
    flex: 1;
  }
  .mt-4 { margin-top: 1rem; }
</style>
