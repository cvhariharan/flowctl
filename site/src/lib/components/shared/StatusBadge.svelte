<script lang="ts">
  let { value }: { value: string } = $props();

  const capitalizeStatus = (status: string) => {
    if (!status) return '';
    return status.charAt(0).toUpperCase() + status.slice(1);
  };

  const getStatusClasses = (status: string) => {
    switch (status) {
      case 'completed':
        return 'bg-success-100 text-success-900';
      case 'approved':
        return 'bg-success-100 text-success-900';
      case 'pending':
        return 'bg-warning-100 text-warning-900';
      case 'running':
        return 'bg-primary-100 text-info-900';
      case 'cancelled':
        return 'bg-warning-100 text-warning-900';
      case 'errored':
        return 'bg-danger-100 text-danger-900';
      case 'rejected':
        return 'bg-danger-100 text-danger-900';
      default:
        return 'bg-gray-100 text-gray-800';
    }
  };

  const getDotClasses = (status: string) => {
    switch (status) {
      case 'completed':
        return 'bg-success-500';
      case 'approved':
        return 'bg-success-500';
      case 'pending':
        return 'bg-warning-500';
      case 'running':
        return 'bg-primary-500';
      case 'cancelled':
        return 'bg-warning-500';
      case 'errored':
        return 'bg-danger-500';
      case 'rejected':
        return 'bg-danger-500';
      default:
        return 'bg-gray-400';
    }
  };
</script>

<span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium {getStatusClasses(value)}">
  <span class="w-1.5 h-1.5 mr-1.5 rounded-full {getDotClasses(value)}"></span>
  <span>{capitalizeStatus(value)}</span>
</span>