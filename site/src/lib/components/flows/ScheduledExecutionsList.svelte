<script lang="ts">
  import type { ScheduledExecution, UserSchedule } from '$lib/types';
  import { getNextCronRun } from '$lib/utils/cronParser';

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
    title = 'Upcoming Scheduled Runs'
  }: {
    schedules: ScheduledExecution[];
    cronSchedules?: UserSchedule[];
    namespace: string;
    flowId: string;
    title?: string;
  } = $props();

  // Compute combined list of upcoming runs
  let upcomingRuns = $derived.by(() => {
    const runs: UpcomingRun[] = [];

    // Add cron-based runs (only active schedules)
    for (const cron of cronSchedules.filter(c => c.is_active)) {
      const nextRun = getNextCronRun(cron.cron, cron.timezone);
      if (nextRun) {
        runs.push({
          type: 'cron',
          name: cron.name,
          label: cron.cron,
          scheduledAt: nextRun
        });
      }
    }

    // Add manually scheduled runs
    for (const schedule of schedules) {
      runs.push({
        type: 'scheduled',
        label: 'Scheduled',
        scheduledAt: new Date(schedule.scheduled_at),
        execId: schedule.exec_id
      });
    }

    // Sort by scheduled time ascending
    return runs.sort((a, b) => a.scheduledAt.getTime() - b.scheduledAt.getTime());
  });

  function formatScheduledTime(date: Date): string {
    return date.toLocaleString();
  }
</script>

{#if upcomingRuns.length > 0}
  <article class="card">
    <header>
      <h3>{title}</h3>
      <p class="text-lighter text-xs">{upcomingRuns.length} {upcomingRuns.length === 1 ? 'run' : 'runs'} scheduled</p>
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
          {/each}
        </tbody>
      </table>
    </div>
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
