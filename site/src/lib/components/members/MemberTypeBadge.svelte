<script lang="ts">
  let { value }: { value: string } = $props();
  
  const isUser = value === 'user';
</script>

<span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-primary-100 text-primary-800">
  {isUser ? 'User' : 'Group'}
</span>