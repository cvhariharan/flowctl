<script lang="ts">
  let { value }: { value: string } = $props();

  function getRoleClass(role: string) {
    switch (role) {
      case 'user': return 'success';
      case 'operator': return 'primary';
      case 'reviewer': return 'warning';
      case 'admin': return 'danger';
      default: return 'primary';
    }
  }

  function capitalizeRole(role: string) {
    return role.charAt(0).toUpperCase() + role.slice(1);
  }
</script>

<span class="badge {getRoleClass(value)}">
  {capitalizeRole(value)}
</span>
