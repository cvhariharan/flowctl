<script lang="ts">
  import type { ScheduledExecution, UserSchedule } from '$lib/types';
  import { getNextCronRun } from '$lib/utils/cronParser';
  import Pagination from '$lib/components/shared/Pagination.svelte';

  interface UpcomingRun {
    type: 'cron' | 'scheduled';
    name?: string;
    label: string;
    scheduledAt: Date;
    execId?: string;
  }

  let {
    schedules = [],
    cronSchedules = [],
    namespace,
    flowId,
    title = 'Upcoming Scheduled Runs',
    currentPage = 1,
    totalPages = 1,
    loading = false,
    onPageChange
  }: {
    schedules: ScheduledExecution[];
    cronSchedules?: UserSchedule[];
    namespace: string;
    flowId: string;
    title?: string;
    currentPage?: number;
    totalPages?: number;
    loading?: boolean;
    onPageChange?: (page: number) => void;
  } = $props();

  let paginated = $derived(totalPages > 1);

  const byTime = (a: UpcomingRun, b: UpcomingRun) => a.scheduledAt.getTime() - b.scheduledAt.getTime();

  // Compute combined list of upcoming runs
  let upcomingRuns = $derived.by(() => {
    const cronRuns: UpcomingRun[] = [];
    const scheduledRuns: UpcomingRun[] = [];

    // Add cron-based runs (only active schedules)
    for (const cron of cronSchedules.filter(c => c.is_active)) {
      const nextRun = getNextCronRun(cron.cron, cron.timezone);
      if (nextRun) {
        cronRuns.push({
          type: 'cron',
          name: cron.name,
          label: cron.cron,
          scheduledAt: nextRun
        });
      }
    }

    // Add manually scheduled runs
    for (const schedule of schedules) {
      scheduledRuns.push({
        type: 'scheduled',
        label: 'Scheduled',
        scheduledAt: new Date(schedule.scheduled_at),
        execId: schedule.exec_id
      });
    }

    return [...scheduledRuns.sort(byTime), ...cronRuns.sort(byTime)];
  });

  function formatScheduledTime(date: Date): string {
    return date.toLocaleString();
  }
</script>

{#if upcomingRuns.length > 0 || paginated}
  <article class="card">
    <header>
      <h3>{title}</h3>
      {#if !paginated}
        <p class="text-lighter text-xs">{upcomingRuns.length} {upcomingRuns.length === 1 ? 'run' : 'runs'} scheduled</p>
      {/if}
    </header>
    <div class="table">
      <table>
        <thead>
          <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Scheduled Time</th>
            <th>Exec ID</th>
          </tr>
        </thead>
        <tbody>
          {#each upcomingRuns as run}
            <tr>
              <td class="name-col">
                <span class="schedule-name" title={run.name || undefined}>{run.name || '-'}</span>
              </td>
              <td>
                {#if run.type === 'cron'}
                  <code>{run.label}</code>
                {:else}
                  {run.label}
                {/if}
              </td>
              <td>
                {formatScheduledTime(run.scheduledAt)}
              </td>
              <td>
                {#if run.execId}
                  <a href="/view/{encodeURIComponent(namespace)}/results/{flowId}/{run.execId}" style="font-family: var(--font-mono); font-size: var(--text-7)">
                    {run.execId.substring(0, 8)}
                  </a>
                {:else}
                  <span class="text-lighter">-</span>
                {/if}
              </td>
            </tr>
          {:else}
            <tr>
              <td colspan="4" class="text-lighter text-sm">No active schedules on this page</td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
    {#if paginated}
      <footer class="hstack justify-end">
        <Pagination
          {currentPage}
          {totalPages}
          {loading}
          on:page-change={(e) => onPageChange?.(e.detail.page)}
        />
      </footer>
    {/if}
  </article>
{/if}

<style>
  .name-col {
    max-width: 14rem;
  }
  .schedule-name {
    display: block;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
