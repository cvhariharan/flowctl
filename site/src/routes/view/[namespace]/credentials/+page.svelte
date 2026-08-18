<script lang="ts">
    import { goto } from "$app/navigation";
    import { browser } from "$app/environment";
    import { page } from "$app/state";
    import type { PageData } from "./$types";
    import PageHeader from "$lib/components/shared/PageHeader.svelte";
    import SearchInput from "$lib/components/shared/SearchInput.svelte";
    import Table from "$lib/components/shared/Table.svelte";
    import Pagination from "$lib/components/shared/Pagination.svelte";
    import CredentialModal from "$lib/components/credentials/CredentialModal.svelte";
    import DeleteModal from "$lib/components/shared/DeleteModal.svelte";
    import { apiClient } from "$lib/apiClient";
    import type { CredentialResp, CredentialReq, CredentialsPaginateResponse } from "$lib/types";
    import { DEFAULT_PAGE_SIZE } from "$lib/constants";
    import Header from "$lib/components/shared/Header.svelte";
    import { handleInlineError, showSuccess } from "$lib/utils/errorHandling";
    import IconPlus from "@tabler/icons-svelte/icons/plus";
    import IconKey from "@tabler/icons-svelte/icons/key";
    import IconShieldCheck from "@tabler/icons-svelte/icons/shield-check";
    import IconLock from "@tabler/icons-svelte/icons/lock";
    import { formatDateTime } from "$lib/utils";
    import NameLinkCell from "$lib/components/shared/cells/NameLinkCell.svelte";
    import BadgeCell from "$lib/components/shared/cells/BadgeCell.svelte";
    import MutedTextCell from "$lib/components/shared/cells/MutedTextCell.svelte";

    let { data }: { data: PageData } = $props();
    const namespace = $derived(page.params.namespace!);
    const encodedNamespace = $derived(encodeURIComponent(namespace));

    // State
    let credentials = $state<CredentialResp[]>([]);
    let totalCount = $state(0);
    let pageCount = $state(0);
    let currentPage = $state(data.currentPage);
    let searchQuery = $state(data.searchQuery);
    let permissions = $state(data.permissions);
    let loading = $state(true);

    // Handle the async data from load function
    $effect(() => {
        let cancelled = false;

        data.credentialsPromise
            .then((result: CredentialsPaginateResponse) => {
                if (!cancelled) {
                    credentials = result.credentials || [];
                    totalCount = result.total_count || 0;
                    pageCount = result.page_count || 1;
                    loading = false;
                }
            })
            .catch((err: Error) => {
                if (!cancelled) {
                    credentials = [];
                    totalCount = 0;
                    pageCount = 0;
                    handleInlineError(err, "Unable to Load Credentials");
                    loading = false;
                }
            });

        return () => {
            cancelled = true;
        };
    });
    let showModal = $state(false);
    let isEditMode = $state(false);
    let editingCredentialId = $state<string | null>(null);
    let editingCredentialData = $state<CredentialResp | null>(null);
    let showDeleteModal = $state(false);
    let deleteCredentialId = $state<string | null>(null);
    let deleteCredentialName = $state("");

    // Table configuration
    let tableColumns = [
        {
            key: "name",
            header: "Name",
            sortable: true,
            component: NameLinkCell,
            componentProps: {
                getIcon: (row: CredentialResp) => row.key_type === "private_key" ? IconShieldCheck : IconLock,
                iconVariant: (row: CredentialResp) => row.key_type === "private_key" ? "success" : "warning",
                subtitleKey: "id",
                onClick: (row: CredentialResp) => handleEdit(row.id)
            }
        },
        {
            key: "key_type",
            header: "Type",
            sortable: true,
            component: BadgeCell,
            componentProps: {
                variant: (row: CredentialResp) => row.key_type === "private_key" ? "success" : "warning",
                label: (row: CredentialResp) => row.key_type === "private_key" ? "SSH Key" : "Password"
            }
        },
        {
            key: "last_accessed",
            header: "Last Accessed",
            sortable: true,
            component: MutedTextCell,
            componentProps: { format: (v: any) => formatDateTime(v, "Never") }
        },
    ];

    let tableActions = $derived(() => {
        const actionsList = [];

        if (permissions.canUpdate) {
            actionsList.push({
                label: "Edit",
                onClick: (credential: CredentialResp) =>
                    handleEdit(credential.id),
                className: "text-link border-link hover:bg-link-hover",
            });
        }

        if (permissions.canDelete) {
            actionsList.push({
                label: "Delete",
                onClick: (credential: CredentialResp) =>
                    handleDelete(credential.id),
                className: "text-danger-600 border-danger-600 hover:bg-danger-100",
            });
        }

        return actionsList;
    });

    // Functions
    async function fetchCredentials(
        filter: string = "",
        pageNumber: number = 1,
    ) {
        if (!browser) return;

        loading = true;
        try {
            const response = await apiClient.credentials.list(data.namespace, {
                page: pageNumber,
                count_per_page: DEFAULT_PAGE_SIZE,
                filter: filter,
            });

            credentials = response.credentials || [];
            totalCount = response.total_count || 0;
            pageCount = response.page_count || 1;
        } catch (error) {
            handleInlineError(error, "Unable to Load Credentials List");
        } finally {
            loading = false;
        }
    }

    function handleSearch(query: string) {
        searchQuery = query;
        fetchCredentials(query);
    }

    function handlePageChange(event: CustomEvent<{ page: number }>) {
        currentPage = event.detail.page;
        fetchCredentials("", currentPage);
    }

    function handleAdd() {
        isEditMode = false;
        editingCredentialId = null;
        editingCredentialData = null;
        showModal = true;
    }

    async function handleEdit(credentialId: string) {
        try {
            loading = true;
            const credential = await apiClient.credentials.getById(
                data.namespace,
                credentialId,
            );

            isEditMode = true;
            editingCredentialId = credentialId;
            editingCredentialData = credential;
            showModal = true;
        } catch (error) {
            handleInlineError(error, "Unable to Load Credential Details");
        } finally {
            loading = false;
        }
    }

    function handleDelete(credentialId: string) {
        const credential = credentials.find((c) => c.id === credentialId);
        if (credential) {
            deleteCredentialId = credentialId;
            deleteCredentialName = credential.name;
            showDeleteModal = true;
        }
    }

    async function confirmDelete() {
        if (!deleteCredentialId) return;

        try {
            await apiClient.credentials.delete(
                data.namespace,
                deleteCredentialId,
            );
            showSuccess(
                "Credential Deleted",
                `Successfully deleted credential "${deleteCredentialName}"`,
            );
            closeDeleteModal(); // Close modal after successful deletion
            await fetchCredentials();
        } catch (error) {
            handleInlineError(error, "Unable to Delete Credential");
        }
    }

    function closeDeleteModal() {
        showDeleteModal = false;
        deleteCredentialId = null;
        deleteCredentialName = "";
    }

    async function handleCredentialSave(credentialData: CredentialReq) {
        try {
            if (isEditMode && editingCredentialId) {
                await apiClient.credentials.update(
                    data.namespace,
                    editingCredentialId,
                    credentialData,
                );
                showSuccess(
                    "Credential Updated",
                    `Successfully updated credential "${credentialData.name}"`,
                );
            } else {
                await apiClient.credentials.create(
                    data.namespace,
                    credentialData,
                );
                showSuccess(
                    "Credential Created",
                    `Successfully created credential "${credentialData.name}"`,
                );
            }
            showModal = false;
            await fetchCredentials();
        } catch (error) {
            handleInlineError(
                error,
                isEditMode
                    ? "Unable to Update Credential"
                    : "Unable to Create Credential",
            );
        }
    }

    function handleModalClose() {
        showModal = false;
        isEditMode = false;
        editingCredentialId = null;
        editingCredentialData = null;
    }

</script>

<svelte:head>
    <title>Credentials - {page.params.namespace} - Flowctl</title>
</svelte:head>

<Header
    breadcrumbs={[
        {
            label: namespace,
            url: `/view/${encodedNamespace}/flows`,
        },
        { label: "Credentials" },
    ]}
>
    {#snippet children()}
        <SearchInput
            bind:value={searchQuery}
            placeholder="Search credentials..."
            {loading}
            onSearch={handleSearch}
        />
    {/snippet}
</Header>

<div class="page-content">
    <!-- Page Header -->
    <PageHeader
        title="Credentials"
        subtitle="Manage SSH keys, passwords, and other authentication credentials"
        actions={permissions.canCreate
            ? [
                  {
                      label: "Add",
                      onClick: handleAdd,
                      variant: "primary",
                      IconComponent: IconPlus,
                      iconSize: 16,
                  },
              ]
            : []}
    />

    <!-- Credentials Table -->
    <div class="mt-4">
        <Table
            data={credentials}
            columns={tableColumns}
            actions={tableActions()}
            {loading}
            emptyMessage="No credentials found. Get started by adding your first credential."
            EmptyIconComponent={IconKey}
            emptyIconSize={64}
            emptyActionLabel={permissions.canCreate ? "Add your first credential" : undefined}
            onEmptyAction={permissions.canCreate ? handleAdd : undefined}
        />
    </div>

    <!-- Pagination -->
    {#if pageCount > 1}
        <Pagination
            {currentPage}
            totalPages={pageCount}
            on:page-change={handlePageChange}
        />
    {/if}
</div>

<!-- Credential Modal -->
{#if showModal}
    <CredentialModal
        {isEditMode}
        credentialData={editingCredentialData}
        onSave={handleCredentialSave}
        onClose={handleModalClose}
    />
{/if}

<!-- Delete Modal -->
{#if showDeleteModal}
    <DeleteModal
        title="Delete Credential"
        description="Deleting this credential will remove any nodes using it"
        itemName={deleteCredentialName}
        onConfirm={confirmDelete}
        onClose={closeDeleteModal}
    />
{/if}
