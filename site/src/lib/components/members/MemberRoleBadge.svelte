<script lang="ts">
  let { value }: { value: string } = $props();
  
  function getRoleClasses(role: string) {
    switch (role) {
      case 'user':
        return 'bg-gray-100 text-gray-800';
      case 'reviewer':
        return 'bg-yellow-100 text-yellow-800';
      case 'admin':
        return 'bg-red-100 text-red-800';
      default:
        return 'bg-gray-100 text-gray-800';
    }
  }
  
  function capitalizeRole(role: string) {
    return role.charAt(0).toUpperCase() + role.slice(1);
  }
</script>

<span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium {getRoleClasses(value)}">
  {capitalizeRole(value)}
</span>