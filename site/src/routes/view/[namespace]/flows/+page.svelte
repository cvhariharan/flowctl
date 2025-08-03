<script lang="ts">
  import { page } from "$app/state";
  import { goto } from "$app/navigation";
  import { apiClient } from "$lib/apiClient";
  import Header from "$lib/components/shared/Header.svelte";
  import Table from "$lib/components/shared/Table.svelte";
  import Pagination from "$lib/components/shared/Pagination.svelte";
  import SearchInput from "$lib/components/shared/SearchInput.svelte";
  import ErrorMessage from "$lib/components/shared/ErrorMessage.svelte";
  import PageHeader from "$lib/components/shared/PageHeader.svelte";
  import type { TableColumn, TableAction, FlowListItem } from "$lib/types";
  import { FLOWS_PER_PAGE } from "$lib/constants";

  let { data } = $props();
  let searchValue = $state("");
  let debounceTimer: number;
  let flows = $state(data.flows);
  let pageCount = $state(data.pageCount);
  let totalCount = $state(data.totalCount);
  let currentPage = $state(data.currentPage);
  let error = $state(data.error);
  let loading = $state(false);

  const goToFlow = (flowSlug: string) => {
    goto(`/view/${page.params.namespace}/flows/${flowSlug}`);
  };

  const loadFlows = async (filter: string = "", pageNumber: number = 1) => {
    loading = true;
    error = "";

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
      error = "Failed to load flows";
      console.error("Failed to load flows:", err);
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

  const actions: TableAction<FlowListItem>[] = [
    {
      label: "Run Flow",
      onClick: (row: FlowListItem) => goToFlow(row.slug),
      className: "text-blue-600 hover:text-blue-700 transition-colors",
    },
  ];
</script>

<svelte:head>
  <title>Flows - {page.params.namespace} - Flowctl</title>
</svelte:head>

<Header breadcrumbs={[`${page.params.namespace}`, "Flows"]}>
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
    </div>

    <!-- Error Message -->
    {#if error}
      <ErrorMessage message={error} />
    {/if}

    <!-- Flows Table -->
    <Table
      {columns}
      data={flows}
      {actions}
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
