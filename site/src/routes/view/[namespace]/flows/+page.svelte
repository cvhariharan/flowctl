<script lang="ts">
    import { page } from "$app/state";
    import { goto } from "$app/navigation";
    import { apiClient } from "$lib/apiClient";
    import Header from "$lib/components/shared/Header.svelte";
    import Table from "$lib/components/shared/Table.svelte";
    import Pagination from "$lib/components/shared/Pagination.svelte";
    import SearchInput from "$lib/components/shared/SearchInput.svelte";
    import PageHeader from "$lib/components/shared/PageHeader.svelte";
    import { handleInlineError, showSuccess, showWarning } from "$lib/utils/errorHandling";
    import { formatDateTime } from "$lib/utils";
    import type {
        TableColumn,
        TableAction,
        FlowListItem,
        FlowsPaginateResponse,
        FlowGroupResp,
    } from "$lib/types";
    import { FLOWS_PER_PAGE } from "$lib/constants";
    import {
        permissionChecker,
        type ResourcePermissions,
    } from "$lib/utils/permissions";
    import DeleteModal from "$lib/components/shared/DeleteModal.svelte";
    import GroupEditModal from "$lib/components/shared/GroupEditModal.svelte";
    import { IconPlus, IconBolt, IconFolder, IconChevronLeft, IconChevronDown } from "@tabler/icons-svelte";
    import NameLinkCell from "$lib/components/shared/cells/NameLinkCell.svelte";
    import MutedTextCell from "$lib/components/shared/cells/MutedTextCell.svelte";

    // Sanity guard only; the server enforces max_flow_import_size.
    const MAX_IMPORT_BYTES = 1024 * 1024;

    type FlowTableRow =
        | (FlowListItem & { _kind: "flow"; flow_count: 0 })
        | (FlowGroupResp & { _kind: "group"; name: string; slug: "" });

    let { data } = $props();
    const namespace = $derived(page.params.namespace!);
    const flowsPath = $derived(
        `/view/${encodeURIComponent(namespace)}/flows`,
    );
    const activeGroup = $derived(data.group || null);
    let searchValue = $state("");
    let flows = $state<FlowListItem[]>([]);
    let groups = $state<FlowGroupResp[]>([]);
    let pageCount = $state(0);
    let totalCount = $state(0);
    let currentPage = $state(1);
    let loading = $state(true);
    let permissions = $state<ResourcePermissions>({
        canCreate: false,
        canRead: false,
        canUpdate: false,
        canDelete: false,
        canViewConfig: false,
        canExport: false,
    });
    let flowToDelete = $state<FlowTableRow | null>(null);
    let groupToEdit = $state<FlowTableRow | null>(null);
    let fileInput = $state<HTMLInputElement>();
    const canCreateRootFlow = $derived(
        permissions.canCreate && !activeGroup,
    );
    const canCreateFirstFlow = $derived(
        canCreateRootFlow && !searchValue,
    );

    const setPaginatedFlows = (
        result: FlowsPaginateResponse,
        pageNumber = currentPage,
    ) => {
        flows = result.flows;
        pageCount = result.page_count;
        totalCount = result.total_count;
        currentPage = pageNumber;
    };

    $effect(() => {
        let cancelled = false;
        const group = data.group;
        loading = true;
        if (!group) searchValue = data.filter;

        const loadInitialData = async () => {
            try {
                if (group && data.groupFlowsPromise) {
                    const result = await data.groupFlowsPromise;
                    if (cancelled) return;

                    flows = result.flows || [];
                    groups = [];
                    pageCount = 0;
                    totalCount = flows.length;
                    currentPage = 1;
                    return;
                }

                const [groupResult, flowResult] = await Promise.all([
                    data.groupsPromise.catch(() => ({ groups: [] })),
                    data.flowsPromise,
                ]);
                if (cancelled) return;

                groups = groupResult.groups || [];
                setPaginatedFlows(flowResult, data.currentPage);
            } catch (err) {
                if (cancelled) return;

                flows = [];
                pageCount = 0;
                totalCount = 0;
                handleInlineError(
                    err,
                    group
                        ? "Unable to Load Group Flows"
                        : "Unable to Load Flows",
                );
            } finally {
                if (!cancelled) loading = false;
            }
        };

        void loadInitialData();

        return () => {
            cancelled = true;
        };
    });

    const confirmDeleteFlow = async () => {
        if (!flowToDelete) return;

        try {
            if (flowToDelete._kind === "group") {
                await apiClient.flows.groups.delete(
                    namespace,
                    flowToDelete.id,
                );
                showSuccess(
                    "Group Deleted",
                    `Group "${flowToDelete.name}" and all its flows have been deleted successfully`,
                );
                const result = await apiClient.flows.groups.list(namespace);
                groups = result.groups || [];
            } else {
                await apiClient.flows.delete(
                    namespace,
                    flowToDelete.slug,
                );
                showSuccess(
                    "Flow Deleted",
                    `Flow "${flowToDelete.name}" has been deleted successfully`,
                );
                if (activeGroup) {
                    await loadGroupFlows(activeGroup);
                } else {
                    await loadFlows(searchValue, currentPage);
                }
            }
        } catch (err) {
            handleInlineError(
                err,
                flowToDelete._kind === "group"
                    ? "Unable to Delete Group"
                    : "Unable to Delete Flow",
            );
        } finally {
            flowToDelete = null;
        }
    };

    const saveGroupEdit = async (data: { description: string }) => {
        if (!groupToEdit) return;

        await apiClient.flows.groups.update(
            namespace,
            groupToEdit.id,
            { name: groupToEdit.prefix, description: data.description },
        );
        showSuccess(
            "Group Updated",
            `Group "${groupToEdit.prefix}" has been updated successfully`,
        );
        const result = await apiClient.flows.groups.list(namespace);
        groups = result.groups || [];
        groupToEdit = null;
    };

    const checkPermissions = async () => {
        permissions = await permissionChecker(
            data.user!,
            "flow",
            data.namespaceId,
            ["create", "update", "delete", "export"],
            "_",
        );
    };

    const handleAdd = () => {
        goto(`${flowsPath}/create`);
    };

    const handleDuplicateFlow = (row: FlowTableRow) => {
        goto(`${flowsPath}/create?duplicate_from=${row.slug}`);
    };

    const closeAddMenu = (action: () => void) => {
        document.getElementById("flows-add-menu")?.hidePopover();
        action();
    };

    const handleExportFlow = async (row: FlowTableRow) => {
        try {
            const definition = await apiClient.flows.export(namespace, row.slug);
            const url = URL.createObjectURL(
                new Blob([definition], { type: "application/yaml" }),
            );
            const link = document.createElement("a");
            link.href = url;
            link.download = `${row.slug}.yaml`;
            document.body.appendChild(link);
            link.click();
            link.remove();
            // Revoking synchronously can abort the download in some browsers.
            setTimeout(() => URL.revokeObjectURL(url), 0);
        } catch (err) {
            handleInlineError(err, "Unable to Export Flow");
        }
    };

    const handleImportFile = async (event: Event) => {
        const input = event.currentTarget as HTMLInputElement;
        const file = input.files?.[0];
        input.value = "";
        if (!file) return;

        if (!/\.(ya?ml|huml)$/i.test(file.name)) {
            showWarning("Invalid File", "Flow definitions must be a .yaml, .yml or .huml file");
            return;
        }
        if (file.size > MAX_IMPORT_BYTES) {
            showWarning("File Too Large", "This file is too large to be a flow definition");
            return;
        }

        try {
            const resp = await apiClient.flows.import(namespace, file);
            showSuccess("Flow Imported", `Flow "${resp.id}" was created successfully`);
            goto(`${flowsPath}/${resp.id}`);
        } catch (err) {
            handleInlineError(err, "Unable to Import Flow");
        }
    };

    void checkPermissions();

    const loadGroupFlows = async (group: string) => {
        loading = true;
        try {
            const result = await apiClient.flows.groups.get(namespace, group);
            flows = result.flows || [];
            totalCount = flows.length;
            pageCount = 0;
        } catch (err) {
            handleInlineError(err, "Unable to Load Group Flows");
        } finally {
            loading = false;
        }
    };

    const loadFlows = async (filter: string = "", pageNumber: number = 1) => {
        loading = true;

        try {
            const result = await apiClient.flows.list(namespace, {
                filter,
                page: pageNumber,
                count_per_page: FLOWS_PER_PAGE,
            });
            setPaginatedFlows(result, pageNumber);
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

    const handlePageChange = (event: CustomEvent<{ page: number }>) => {
        loadFlows(searchValue.trim(), event.detail.page);
    };

    const isSearching = $derived(searchValue.trim().length > 0);
    const toFlowRow = (flow: FlowListItem): FlowTableRow => ({
        ...flow,
        _kind: "flow",
        flow_count: 0,
    });

    const tableData = $derived.by(() => {
        if (activeGroup || isSearching) {
            return flows.map(toFlowRow);
        }

        const rows: FlowTableRow[] = [];
        const groupMeta = new Map(groups.map((group) => [group.prefix, group]));
        const seenPrefixes = new Set<string>();

        for (const flow of flows) {
            if (!flow.prefix) {
                rows.push(toFlowRow(flow));
                continue;
            }

            if (seenPrefixes.has(flow.prefix)) continue;

            seenPrefixes.add(flow.prefix);
            const group = groupMeta.get(flow.prefix);
            rows.push({
                _kind: "group",
                name: flow.prefix,
                description: group?.description || "",
                prefix: flow.prefix,
                flow_count: group?.flow_count ?? 0,
                slug: "",
                id: group?.id || "",
            });
        }

        return rows;
    });

    const columns: TableColumn<FlowTableRow>[] = [
        {
            key: "name",
            header: "Name",
            sortable: true,
            component: NameLinkCell,
            componentProps: {
                getIcon: (row: FlowTableRow) =>
                    row._kind === "group" ? IconFolder : IconBolt,
                href: (row: FlowTableRow) =>
                    row._kind === "group"
                        ? `${flowsPath}?group=${encodeURIComponent(row.prefix)}`
                        : `${flowsPath}/${row.slug}`,
                subtitle: (row: FlowTableRow) =>
                    row._kind === "group"
                        ? `${row.flow_count} flow${row.flow_count !== 1 ? "s" : ""}`
                        : row.prefix || undefined,
            },
        },
        {
            key: "description",
            header: "Description",
            component: MutedTextCell,
            componentProps: { truncate: 40 },
        },
        {
            key: "next_run",
            header: "Next run",
            component: MutedTextCell,
            componentProps: { format: (v: any) => formatDateTime(v, "-") },
        },
    ];

    const actions = $derived.by(() => {
        const actionsList: TableAction<FlowTableRow>[] = [];

        if (permissions.canUpdate) {
            actionsList.push({
                label: "Edit",
                onClick: (row) => {
                    if (row._kind === "group") {
                        groupToEdit = row;
                    } else {
                        goto(`${flowsPath}/${row.slug}/edit`);
                    }
                },
                className: "text-link",
            });
        }

        if (permissions.canCreate) {
            actionsList.push({
                label: "Duplicate",
                onClick: handleDuplicateFlow,
                visible: (row) => row._kind === "flow",
            });
        }

        if (permissions.canExport) {
            actionsList.push({
                label: "Export",
                onClick: handleExportFlow,
                visible: (row) => row._kind === "flow",
            });
        }

        if (permissions.canDelete) {
            actionsList.push({
                label: "Delete",
                onClick: (row) => (flowToDelete = row),
                className: "text-danger-600",
            });
        }

        return actionsList;
    });

    const breadcrumbs = $derived(
        [
            { label: namespace },
            { label: "Flows", url: flowsPath },
            ...(activeGroup ? [{ label: activeGroup }] : []),
        ],
    );
</script>

<svelte:head>
    <title>{activeGroup ? `${activeGroup} - ` : ""}Flows - {namespace} - Flowctl</title>
</svelte:head>

<Header breadcrumbs={breadcrumbs}>
    {#snippet children()}
        {#if !activeGroup}
            <SearchInput
                bind:value={searchValue}
                placeholder="Search flows..."
                {loading}
                onSearch={handleSearch}
            />
        {/if}
    {/snippet}
</Header>

<div class="page-content">
    <PageHeader
        title={activeGroup || "Flows"}
        subtitle={activeGroup
            ? `Flows in the ${activeGroup} group`
            : "Manage and run your workflows"}
    >
        {#snippet children()}
            {#if canCreateRootFlow}
                <ot-dropdown>
                    <button popovertarget="flows-add-menu" aria-label="Add" class="hstack gap-2">
                        <IconPlus size={16} aria-hidden="true" />
                        Add
                        <IconChevronDown size={14} aria-hidden="true" />
                    </button>
                    <div popover id="flows-add-menu" role="menu">
                        <button role="menuitem" onclick={() => closeAddMenu(handleAdd)}>Create in UI</button>
                        <button role="menuitem" onclick={() => closeAddMenu(() => fileInput?.click())}>Import from file</button>
                    </div>
                </ot-dropdown>
            {/if}
        {/snippet}
    </PageHeader>

    {#if canCreateRootFlow}
        <input
            bind:this={fileInput}
            type="file"
            accept=".yaml,.yml,.huml,application/yaml,text/yaml"
            style="display: none"
            onchange={handleImportFile}
        />
    {/if}

    {#if activeGroup}
        <button
            onclick={() => goto(flowsPath)}
            class="back-btn mb-4"
            data-variant="secondary"
        >
            <IconChevronLeft size={16} />
            Back to all flows
        </button>
    {/if}

    <Table
        {columns}
        data={tableData}
        actions={actions}
        {loading}
        emptyMessage={activeGroup
            ? `No flows in the "${activeGroup}" group`
            : searchValue
                ? "Try adjusting your search"
                : "No flows are available in this namespace"}
        emptyActionLabel={canCreateFirstFlow ? "Create your first flow" : undefined}
        onEmptyAction={canCreateFirstFlow ? handleAdd : undefined}
        EmptyIconComponent={IconBolt}
        emptyIconSize={48}
    />

    {#if !activeGroup && flows.length > 0}
        <div class="mt-6 hstack justify-between items-center">
            <div class="text-light text-sm">
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

    {#if activeGroup && flows.length > 0}
        <div class="mt-6 text-light text-sm">
            {flows.length} flow{flows.length !== 1 ? "s" : ""} in this group
        </div>
    {/if}
</div>

{#if flowToDelete}
    <DeleteModal
        title={flowToDelete._kind === "group" ? "Delete Group" : "Delete Flow"}
        itemName={flowToDelete.name}
        description={flowToDelete._kind === "group"
            ? "This will permanently delete all flows in this group."
            : null}
        onConfirm={confirmDeleteFlow}
        onClose={() => (flowToDelete = null)}
    />
{/if}

{#if groupToEdit}
    <GroupEditModal
        groupName={groupToEdit.prefix}
        description={groupToEdit.description}
        onSave={saveGroupEdit}
        onClose={() => (groupToEdit = null)}
    />
{/if}
