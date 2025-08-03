<script lang="ts">
  let { value }: { value: string } = $props();
  
  const isUser = value === 'user';
</script>

<span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium {isUser ? 'bg-blue-100 text-blue-800' : 'bg-purple-100 text-purple-800'}">
  {isUser ? 'User' : 'Group'}
</span>