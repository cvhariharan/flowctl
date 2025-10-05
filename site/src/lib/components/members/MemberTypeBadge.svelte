<script lang="ts">
  let { value }: { value: string } = $props();
  
  const isUser = value === 'user';
</script>

<span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium {isUser ? 'bg-info-100 text-info-900' : 'bg-primary-100 text-primary-900'}">
  {isUser ? 'User' : 'Group'}
</span>