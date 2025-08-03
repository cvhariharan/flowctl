<script lang="ts">
  let { value }: { value: string } = $props();

  const capitalizeStatus = (status: string) => {
    if (!status) return '';
    return status.charAt(0).toUpperCase() + status.slice(1);
  };

  const getStatusClasses = (status: string) => {
    switch (status) {
      case 'completed':
        return 'bg-green-100 text-green-800';
      case 'approved':
        return 'bg-green-100 text-green-800';
      case 'pending':
        return 'bg-yellow-100 text-yellow-800';
      case 'running':
        return 'bg-blue-100 text-blue-800';
      case 'errored':
        return 'bg-red-100 text-red-800';
      case 'rejected':
        return 'bg-red-100 text-red-800';
      default:
        return 'bg-gray-100 text-gray-800';
    }
  };

  const getDotClasses = (status: string) => {
    switch (status) {
      case 'completed':
        return 'bg-green-400';
      case 'approved':
        return 'bg-green-400';
      case 'pending':
        return 'bg-yellow-400';
      case 'running':
        return 'bg-blue-400';
      case 'errored':
        return 'bg-red-400';
      case 'rejected':
        return 'bg-red-400';
      default:
        return 'bg-gray-400';
    }
  };
</script>

<span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium {getStatusClasses(value)}">
  <span class="w-1.5 h-1.5 mr-1.5 rounded-full {getDotClasses(value)}"></span>
  <span>{capitalizeStatus(value)}</span>
</span>