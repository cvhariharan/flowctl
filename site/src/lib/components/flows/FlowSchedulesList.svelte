<script lang="ts">
  import { handleInlineError, showSuccess } from '$lib/utils/errorHandling';
  import { apiClient } from '$lib/apiClient';
  import ScheduleModal from './ScheduleModal.svelte';
  import ViewScheduleModal from './ViewScheduleModal.svelte';
  import DeleteModal from '$lib/components/shared/DeleteModal.svelte';
  import { IconClock, IconPlus, IconDotsVertical } from '@tabler/icons-svelte';
  import type { UserSchedule, FlowInput, ScheduleCreateReq, ScheduleUpdateReq } from '$lib/types';

  let {
    namespace,
    flowId,
    flowInputs,
    userSchedulable = false,
    user,
    schedules = [],
    onUpdate,
    canUpdateFlow = false
  }: {
    namespace: string;
    flowId: string;
    flowInputs: FlowInput[];
    userSchedulable: boolean;
    user: any;
    schedules?: UserSchedule[];
    onUpdate?: () => Promise<void>;
    canUpdateFlow?: boolean;
  } = $props();

  let showModal = $state(false);
  let showDelete = $state(false);
  let showViewModal = $state(false);
  let editSchedule = $state<UserSchedule | null>(null);
  let deleteSchedule = $state<UserSchedule | null>(null);
  let viewSchedule = $state<UserSchedule | null>(null);
  let canCreateSchedule = $derived(userSchedulable);

  function canEdit(schedule: UserSchedule): boolean {
    return canUpdateFlow || (schedule.is_user_created && schedule.created_by === user.id);
  }

  async function handleSave(data: ScheduleCreateReq | ScheduleUpdateReq) {
    if (editSchedule) {
      await apiClient.flows.schedules.update(namespace, flowId, editSchedule.uuid, data);
    } else {
      await apiClient.flows.schedules.create(namespace, flowId, data as ScheduleCreateReq);
    }
    if (onUpdate) await onUpdate();
  }

  async function handleDelete() {
    if (!deleteSchedule) return;
    await apiClient.flows.schedules.delete(namespace, flowId, deleteSchedule.uuid);
    showSuccess('Schedule Deleted', 'Schedule deleted successfully');
    showDelete = false;
    deleteSchedule = null;
    if (onUpdate) await onUpdate();
  }

  async function toggleActive(schedule: UserSchedule) {
    try {
      await apiClient.flows.schedules.update(namespace, flowId, schedule.uuid, {
        cron: schedule.cron,
        timezone: schedule.timezone,
        inputs: schedule.inputs,
        is_active: !schedule.is_active
      });
      showSuccess('Updated', 'Schedule status updated');
      if (onUpdate) await onUpdate();
    } catch (error) {
      handleInlineError(error, 'Failed to update schedule');
    }
  }

  function getMenuItems(schedule: UserSchedule) {
    return [
      {
        label: 'View',
        onClick: () => { viewSchedule = schedule; showViewModal = true; }
      },
      {
        label: 'Edit',
        onClick: () => { editSchedule = schedule; showModal = true; }
      },
      {
        label: schedule.is_active ? 'Deactivate' : 'Activate',
        onClick: () => toggleActive(schedule)
      },
      {
        label: 'Delete',
        onClick: () => { deleteSchedule = schedule; showDelete = true; },
        variant: 'danger' as const
      }
    ];
  }
</script>

<article class="card">
  <header class="hstack justify-between">
    <div>
      <h3>Schedules</h3>
      <p class="text-lighter text-xs">{schedules.length} {schedules.length === 1 ? 'schedule' : 'schedules'}</p>
    </div>
    {#if canCreateSchedule}
      <button
        type="button"
        onclick={() => { editSchedule = null; showModal = true; }}
      >
        <IconPlus size={16} />
        Add
      </button>
    {/if}
  </header>

  {#if schedules.length === 0}
    <div class="vstack empty-state">
      <IconClock size={48} class="text-lighter" />
      <p class="text-light">No schedules configured</p>
      {#if canCreateSchedule}
        <button
          type="button"
          data-variant="secondary"
          onclick={() => { editSchedule = null; showModal = true; }}
        >
          Create your first schedule
        </button>
      {/if}
    </div>
  {:else}
    <div class="table">
      <table>
        <thead>
          <tr>
            <th>Cron</th>
            <th>Timezone</th>
            <th>Type</th>
            <th>Status</th>
            <th class="actions-col">Actions</th>
          </tr>
        </thead>
        <tbody>
          {#each schedules as schedule}
            <tr>
              <td>
                <code>{schedule.cron}</code>
              </td>
              <td>{schedule.timezone}</td>
              <td>
                <span class="badge {schedule.is_user_created ? 'success' : 'info'}">
                  {schedule.is_user_created ? 'User' : 'System'}
                </span>
              </td>
              <td>
                {#if schedule.is_user_created}
                  <span class="badge {schedule.is_active ? 'success' : 'primary'}">
                    {schedule.is_active ? 'Active' : 'Inactive'}
                  </span>
                {:else}
                  <span class="text-lighter">-</span>
                {/if}
              </td>
              <td class="actions-col">
                {#if canCreateSchedule && canEdit(schedule)}
                  {@const menuId = `sched-menu-${schedule.uuid}`}
                  <div class="actions-wrapper">
                    <ot-dropdown>
                      <button popovertarget={menuId} class="ghost icon small" aria-label="Actions menu">
                        <IconDotsVertical size={16} />
                      </button>
                      <div popover id={menuId} role="menu">
                        {#each getMenuItems(schedule) as item}
                          <button
                            role="menuitem"
                            onclick={() => { item.onClick(); document.getElementById(menuId)?.hidePopover(); }}
                            style={item.variant === 'danger' ? 'color: var(--danger)' : ''}
                          >
                            {item.label}
                          </button>
                        {/each}
                      </div>
                    </ot-dropdown>
                  </div>
                {:else}
                  <span class="text-lighter">-</span>
                {/if}
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</article>

{#if showModal}
  <ScheduleModal
    isEditMode={!!editSchedule}
    schedule={editSchedule}
    {flowInputs}
    {namespace}
    {flowId}
    onSave={handleSave}
    onClose={() => { showModal = false; editSchedule = null; }}
  />
{/if}

{#if showDelete && deleteSchedule}
  <DeleteModal
    title="Delete Schedule"
    itemName={`${deleteSchedule.cron} (${deleteSchedule.timezone})`}
    onConfirm={handleDelete}
    onClose={() => { showDelete = false; deleteSchedule = null; }}
  />
{/if}

{#if showViewModal && viewSchedule}
  <ViewScheduleModal
    schedule={viewSchedule}
    onClose={() => { showViewModal = false; viewSchedule = null; }}
  />
{/if}

<style>
  .empty-state {
    align-items: center;
    padding: var(--space-12) var(--space-4);
    gap: var(--space-3);
  }
  .actions-col {
    text-align: right;
    width: 5rem;
  }
  .actions-wrapper {
    display: inline-flex;
    justify-content: flex-end;
  }
</style>
