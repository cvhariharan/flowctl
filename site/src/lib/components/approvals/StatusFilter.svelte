<script lang="ts">
  let { 
    value = $bindable(''),
    onChange
  }: { 
    value: string;
    onChange: (status: string) => void;
  } = $props();

  function handleChange(event: Event) {
    const target = event.target as HTMLSelectElement;
    value = target.value;
    onChange(target.value);
  }
</script>

<div class="relative">
  <select 
    {value}
    onchange={handleChange}
    class="appearance-none bg-white border border-gray-300 rounded-lg px-4 py-2 pr-8 text-sm focus:ring-2 focus:ring-primary-500 focus:border-primary-500"
  >
    <option value="">All Statuses</option>
    <option value="pending">Pending</option>
    <option value="approved">Approved</option>
    <option value="rejected">Rejected</option>
  </select>
  <svg class="w-4 h-4 absolute right-3 top-1/2 transform -translate-y-1/2 text-gray-400 pointer-events-none" fill="none" stroke="currentColor" viewBox="0 0 24 24">
    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7"/>
  </svg>
</div>