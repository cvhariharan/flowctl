<script lang="ts">
  import { page } from "$app/state";
  import { goto } from "$app/navigation";
  import { apiClient } from "$lib/apiClient";
  import Header from "$lib/components/shared/Header.svelte";
  import Table from "$lib/components/shared/Table.svelte";
  import Pagination from "$lib/components/shared/Pagination.svelte";
  import SearchInput from "$lib/components/shared/SearchInput.svelte";
  import PageHeader from "$lib/components/shared/PageHeader.svelte";
  import { handleInlineError, showSuccess } from "$lib/utils/errorHandling";
  import type { TableColumn, TableAction, FlowListItem } from "$lib/types";
  import { FLOWS_PER_PAGE } from "$lib/constants";
  import { permissionChecker, type ResourcePermissions } from "$lib/utils/permissions";
  import DeleteModal from "$lib/components/shared/DeleteModal.svelte";

  let { data } = $props();
  let searchValue = $state("");
  let flows = $state(data.flows);
  let pageCount = $state(data.pageCount);
  let totalCount = $state(data.totalCount);
  let currentPage = $state(data.currentPage);
  let loading = $state(false);
  let permissions = $state<ResourcePermissions>({
    canCreate: false,
    canRead: false,
    canUpdate: false,
    canDelete: false
  });
  let showDeleteModal = $state(false);
  let flowToDelete = $state<FlowListItem | null>(null);

  const goToFlow = (flowSlug: string) => {
    goto(`/view/${page.params.namespace}/flows/${flowSlug}`);
  };

  const goToEditFlow = (flowSlug: string) => {
    goto(`/view/${page.params.namespace}/flows/${flowSlug}/edit`);
  };

  const handleDeleteFlow = (flow: FlowListItem) => {
    flowToDelete = flow;
    showDeleteModal = true;
  };

  const confirmDeleteFlow = async () => {
    if (!flowToDelete) return;
    
    try {
      await apiClient.flows.delete(page.params.namespace!, flowToDelete.slug);
      showSuccess('Flow Deleted', `Flow "${flowToDelete.name}" has been deleted successfully`);
      await loadFlows(searchValue, currentPage);
    } catch (err) {
      handleInlineError(err, 'Unable to Delete Flow');
    } finally {
      showDeleteModal = false;
      flowToDelete = null;
    }
  };

  const cancelDelete = () => {
    showDeleteModal = false;
    flowToDelete = null;
  };

  // Check permissions on mount
  const checkPermissions = async () => {
    permissions = await permissionChecker(data.user!, 'flow', data.namespaceId, ['create', 'update', 'delete']);
  };

  checkPermissions();

  const loadFlows = async (filter: string = "", pageNumber: number = 1) => {
    loading = true;

    try {
      const result = await apiClient.flows.list(page.params.namespace!, {
        filter,
        page: pageNumber,
        count_per_page: FLOWS_PER_PAGE,
      });

      flows = result.flows;
      pageCount = result.page_count;
      totalCount = result.total_count;
      currentPage = pageNumber;
    } catch (err) {
      handleInlineError(err, "Unable to Load Flows List");
    } finally {
      loading = false;
    }
  };

  const handleSearch = (query: string) => {
    searchValue = query;
    loadFlows(query, 1);
  };

  const goToPage = (pageNum: number) => {
    loadFlows(searchValue.trim(), pageNum);
  };

  const handlePageChange = (event: CustomEvent<{ page: number }>) => {
    goToPage(event.detail.page);
  };

  const columns: TableColumn<FlowListItem>[] = [
    {
      key: "name",
      header: "Flow Name",
      sortable: true,
      render: (value: string, row: FlowListItem) => `
        <div class="flex items-center">
          <div class="flex-shrink-0 h-8 w-8 bg-blue-100 rounded-lg flex items-center justify-center">
            <svg class="w-4 h-4 text-blue-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z"></path>
            </svg>
          </div>
          <div class="ml-4">
            <div class="text-sm font-medium text-gray-900">${value}</div>
          </div>
        </div>
      `,
    },
    {
      key: "description",
      header: "Description",
      render: (value: string) =>
        `<div class="text-sm text-gray-600 max-w-xs truncate">${value}</div>`,
    },
    {
      key: "schedule",
      header: "Schedule",
      sortable: true,
      render: (value: string) => {
        if (!value) {
          return `<span class="text-sm text-gray-400 italic">Manual only</span>`;
        }
        return `
          <div class="flex items-center text-sm">
            <svg class="w-4 h-4 text-gray-400 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"></path>
            </svg>
            <code class="text-xs bg-gray-100 px-1 py-0.5 rounded">${value}</code>
          </div>
        `;
      },
    },
    {
      key: "step_count",
      header: "Steps",
      render: (value: number) => `
        <div class="flex items-center text-sm text-gray-500">
          <span>${value || 0}</span>
          <span class="ml-1">steps</span>
        </div>
      `,
    },
  ];

  const actions = $derived(() => {
    const actionsList: TableAction<FlowListItem>[] = [];

    if (permissions.canUpdate) {
      actionsList.push({
        label: "Edit",
        onClick: (row: FlowListItem) => goToEditFlow(row.slug),
        className: "text-blue-600 hover:text-blue-700 transition-colors cursor-pointer",
      });
    }

    if (permissions.canDelete) {
      actionsList.push({
        label: "Delete",
        onClick: (row: FlowListItem) => handleDeleteFlow(row),
        className: "text-red-600 hover:text-red-700 transition-colors cursor-pointer",
      });
    }

    return actionsList;
  });
</script>

<svelte:head>
  <title>Flows - {page.params.namespace} - Flowctl</title>
</svelte:head>

<Header breadcrumbs={[
  { label: page.params.namespace! },
  { label: "Flows" }
]}>
  {#snippet children()}
    <SearchInput
      bind:value={searchValue}
      placeholder="Search flows..."
      {loading}
      onSearch={handleSearch}
    />
  {/snippet}
</Header>

<!-- Page Content -->
<div class="flex-1 overflow-y-auto p-6 bg-gray-50">
  <div class="max-w-7xl mx-auto">
    <div class="flex items-center justify-between mb-6">
      <PageHeader title="Flows" subtitle="Manage and run your workflows" />
      {#if permissions.canCreate}
        <button
          class="inline-flex items-center px-4 py-2 text-sm font-medium text-white bg-blue-600 border border-transparent rounded-md shadow-sm hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500"
          onclick={() => goto(`/view/${page.params.namespace}/flows/create`)}
        >
          <svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"></path>
          </svg>
          Add Flow
        </button>
      {/if}
    </div>


    <!-- Flows Table -->
    <Table
      {columns}
      data={flows}
      actions={actions()}
      {loading}
      onRowClick={(row) => goToFlow(row.slug)}
      emptyMessage={searchValue
        ? "Try adjusting your search"
        : "No flows are available in this namespace"}
      emptyIcon={`
        <svg class="mx-auto h-12 w-12 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z"></path>
        </svg>
      `}
    />

    <!-- Pagination and Count -->
    {#if flows.length > 0}
      <div class="mt-6 flex items-center justify-between">
        <div class="text-sm text-gray-700">
          Showing {flows.length} of {totalCount} flows
        </div>
        <Pagination
          {currentPage}
          totalPages={pageCount}
          {loading}
          on:page-change={handlePageChange}
        />
      </div>
    {/if}
  </div>
</div>

<!-- Delete Modal -->
{#if showDeleteModal && flowToDelete}
  <DeleteModal
    title="Delete Flow"
    itemName={flowToDelete.name}
    onConfirm={confirmDeleteFlow}
    onClose={cancelDelete}
  />
{/if}
