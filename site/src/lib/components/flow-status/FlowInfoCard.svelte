<script lang="ts">
  let { 
    flowName,
    startTime,
    executionId,
    status
  }: {
    flowName: string,
    startTime: string,
    executionId: string,
    status?: string
  } = $props();
</script>

<!-- Flow Info Card -->
<div class="bg-white rounded-lg shadow-sm border border-gray-200 p-6 mb-6">
  <div class="flex justify-between items-start">
    <div>
      <h1 class="text-2xl font-semibold text-gray-900">{flowName}</h1>
      <p class="text-gray-600 mt-1">Started at {startTime}</p>
    </div>
    <div class="text-right">
      <p class="text-sm text-gray-500">Execution ID</p>
      <p class="font-mono text-sm text-gray-900">{executionId}</p>
    </div>
  </div>
</div>