<script lang="ts">
  let {
    flowName,
    startTime,
    executionId,
    status,
    scheduledAt,
    triggerType,
    triggeredBy
  }: {
    flowName: string,
    startTime: string,
    executionId: string,
    status?: string,
    scheduledAt?: string,
    triggerType?: string,
    triggeredBy?: string
  } = $props();

  function extractName(triggeredBy: string): string {
    const match = triggeredBy.match(/^(.+?)\s*</);
    return match ? match[1].trim() : triggeredBy;
  }
</script>

<article class="card mb-4">
  <div class="row" style="align-items: start">
    <!-- Left: title, badge, started -->
    <div class="col-6">
      <div class="hstack gap-3 items-center">
        <h1 style="font-size: var(--text-4); margin: 0">{flowName}</h1>
        {#if triggerType}
          <span class="badge {triggerType === 'manual' ? 'primary' : 'success'}">
            {triggerType}
          </span>
        {/if}
      </div>
      <p class="text-sm text-light mt-2" style="margin-bottom: 0">Started at {startTime}</p>
    </div>
    <!-- Right: exec id, triggered by -->
    <div class="col-6 align-right">
      <div class="hstack gap-2 text-sm justify-end">
        <span class="text-light">Execution ID</span>
        <span class="font-mono">{executionId}</span>
      </div>
      {#if triggeredBy}
        <div class="hstack gap-2 text-sm mt-2 justify-end">
          <span class="text-light">Triggered by</span>
          <span>{extractName(triggeredBy)}</span>
        </div>
      {/if}
      {#if scheduledAt}
        <div class="hstack gap-2 text-sm mt-2 justify-end">
          <span class="text-light">Scheduled at</span>
          <span>{scheduledAt}</span>
        </div>
      {/if}
    </div>
  </div>
</article>
