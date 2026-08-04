<script lang="ts">
  let { value }: { value: string } = $props();

  const capitalizeStatus = (status: string) => {
    if (!status) return '';
    return status.split('_').map(w => w.charAt(0).toUpperCase() + w.slice(1)).join(' ');
  };

  const getBadgeClass = (status: string) => {
    switch (status) {
      case 'completed':
      case 'approved':
        return 'success';
      case 'pending':
      case 'pending_approval':
        return 'warning';
      case 'cancelled':
        return 'neutral';
      case 'running':
        return 'info';
      case 'errored':
      case 'rejected':
        return 'danger';
      default:
        return 'primary';
    }
  };
</script>

<span class="badge {getBadgeClass(value)}">
  {capitalizeStatus(value)}
</span>
