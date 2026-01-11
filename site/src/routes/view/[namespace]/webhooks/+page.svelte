<script lang="ts">
    import { browser } from "$app/environment";
    import { page } from "$app/state";
    import type { PageData } from "./$types";
    import Header from "$lib/components/shared/Header.svelte";
    import PageHeader from "$lib/components/shared/PageHeader.svelte";
    import Table from "$lib/components/shared/Table.svelte";
    import DeleteModal from "$lib/components/shared/DeleteModal.svelte";
    import WebhookModal from "$lib/components/webhooks/WebhookModal.svelte";
    import WebhookDuplicateModal from "$lib/components/webhooks/WebhookDuplicateModal.svelte";
    import { apiClient } from "$lib/apiClient";
    import type {
        WebhookCreateReq,
        WebhookUpdateReq,
        WebhookListItem,
        WebhookListResponse,
        WebhookResp,
    } from "$lib/types";
    import { handleInlineError, showSuccess } from "$lib/utils/errorHandling";
    import { formatDateTime } from "$lib/utils";
    import {
        IconCopy,
        IconPencil,
        IconPlus,
        IconSend,
        IconTrash,
        IconWebhook,
    } from "@tabler/icons-svelte";

    let { data }: { data: PageData } = $props();

    let webhooks = $state<WebhookListItem[]>([]);
    let loading = $state(true);
    let showModal = $state(false);
    let showDeleteModal = $state(false);
    let isEditMode = $state(false);
    let selectedWebhook = $state<WebhookResp | null>(null);
    let deleteTarget = $state<WebhookListItem | null>(null);
    let showDuplicateModal = $state(false);
    let duplicateTarget = $state<WebhookListItem | null>(null);

    $effect(() => {
        let cancelled = false;

        data.webhooksPromise
            .then((result: WebhookListResponse) => {
                if (!cancelled) {
                    webhooks = result.webhooks || [];
                    loading = false;
                }
            })
            .catch((err: Error) => {
                if (!cancelled) {
                    webhooks = [];
                    handleInlineError(err, "Unable to Load Webhooks");
                    loading = false;
                }
            });

        return () => {
            cancelled = true;
        };
    });

    async function fetchWebhooks() {
        if (!browser) return;
        loading = true;
        try {
            const response = await apiClient.namespaceWebhooks.list(
                data.namespace,
            );
            webhooks = response.webhooks || [];
        } catch (error) {
            handleInlineError(error, "Unable to Load Webhooks");
        } finally {
            loading = false;
        }
    }

    function handleAdd() {
        selectedWebhook = null;
        isEditMode = false;
        showModal = true;
    }

    async function handleEdit(webhook: WebhookListItem) {
        try {
            const details = await apiClient.namespaceWebhooks.getById(
                data.namespace,
                webhook.id,
            );
            selectedWebhook = details;
            isEditMode = true;
            showModal = true;
        } catch (error) {
            handleInlineError(error, "Unable to Load Webhook");
        }
    }

    async function handleSave(webhookData: WebhookCreateReq | WebhookUpdateReq) {
        try {
            if (isEditMode && selectedWebhook) {
                await apiClient.namespaceWebhooks.update(
                    data.namespace,
                    selectedWebhook.id,
                    webhookData as WebhookUpdateReq,
                );
                showSuccess("Webhook Updated", "Webhook updated successfully");
            } else {
                await apiClient.namespaceWebhooks.create(
                    data.namespace,
                    webhookData as WebhookCreateReq,
                );
                showSuccess("Webhook Created", "Webhook created successfully");
            }

            showModal = false;
            await fetchWebhooks();
        } catch (error) {
            handleInlineError(
                error,
                isEditMode ? "Unable to Update Webhook" : "Unable to Create Webhook",
            );
        }
    }

    async function handleDelete(webhook: WebhookListItem) {
        deleteTarget = webhook;
        showDeleteModal = true;
    }

    async function confirmDelete() {
        if (!deleteTarget) return;
        try {
            await apiClient.namespaceWebhooks.delete(
                data.namespace,
                deleteTarget.id,
            );
            showSuccess("Webhook Deleted", "Webhook deleted successfully");
            closeDeleteModal();
            await fetchWebhooks();
        } catch (error) {
            handleInlineError(error, "Unable to Delete Webhook");
        }
    }

    async function handleTest(webhook: WebhookListItem) {
        try {
            await apiClient.namespaceWebhooks.test(data.namespace, webhook.id);
            showSuccess("Test Sent", "Webhook test sent successfully");
        } catch (error) {
            handleInlineError(error, "Unable to Test Webhook");
        }
    }

    async function handleDuplicate(webhook: WebhookListItem) {
        duplicateTarget = webhook;
        showDuplicateModal = true;
    }

    async function confirmDuplicate(payload: {
        name: string;
        description?: string;
    }) {
        if (!duplicateTarget) return;
        try {
            await apiClient.namespaceWebhooks.duplicate(
                data.namespace,
                duplicateTarget.id,
                payload,
            );
            showSuccess("Webhook Duplicated", "Webhook duplicated successfully");
            closeDuplicateModal();
            await fetchWebhooks();
        } catch (error) {
            handleInlineError(error, "Unable to Duplicate Webhook");
        }
    }

    function closeDeleteModal() {
        showDeleteModal = false;
        deleteTarget = null;
    }

    function closeDuplicateModal() {
        showDuplicateModal = false;
        duplicateTarget = null;
    }

    function handleModalClose() {
        showModal = false;
        isEditMode = false;
        selectedWebhook = null;
    }

    const tableColumns = [
        {
            key: "name",
            header: "Name",
            sortable: true,
            render: (_value: any, webhook: WebhookListItem) => `
                <div>
                    <div class="text-sm font-medium text-gray-900">${webhook.name}</div>
                    ${webhook.description ? `<div class="text-xs text-gray-500">${webhook.description}</div>` : ""}
                    <div class="text-xs text-gray-400">${webhook.url_masked}</div>
                </div>
            `,
        },
        {
            key: "type",
            header: "Type",
            sortable: true,
            render: (_value: any, webhook: WebhookListItem) => {
                const label = webhook.type.charAt(0).toUpperCase() + webhook.type.slice(1);
                const classes =
                    webhook.type === "slack"
                        ? "bg-blue-100 text-blue-800"
                        : webhook.type === "teams"
                          ? "bg-indigo-100 text-indigo-800"
                          : "bg-gray-100 text-gray-700";
                return `<span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${classes}">${label}</span>`;
            },
        },
        {
            key: "is_active",
            header: "Status",
            sortable: true,
            render: (_value: any, webhook: WebhookListItem) => {
                const label = webhook.is_active ? "Active" : "Disabled";
                const classes = webhook.is_active
                    ? "bg-success-100 text-success-800"
                    : "bg-gray-100 text-gray-600";
                return `<span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${classes}">${label}</span>`;
            },
        },
        {
            key: "updated_at",
            header: "Updated",
            sortable: true,
            render: (_value: any, webhook: WebhookListItem) =>
                `<div class="text-sm text-gray-600">${formatDateTime(webhook.updated_at)}</div>`,
        },
    ];

    let tableActions = $derived(() => {
        const actionsList = [];

        if (data.permissions?.canUpdate) {
            actionsList.push({
                label: "Edit",
                onClick: (webhook: WebhookListItem) => handleEdit(webhook),
                className:
                    "text-primary-600 border-primary-600 hover:bg-primary-50 p-2",
                IconComponent: IconPencil,
                iconOnly: true,
            });
            actionsList.push({
                label: "Test",
                onClick: (webhook: WebhookListItem) => handleTest(webhook),
                className:
                    "text-indigo-600 border-indigo-600 hover:bg-indigo-50 p-2",
                IconComponent: IconSend,
                iconOnly: true,
            });
        }

        if (data.permissions?.canCreate) {
            actionsList.push({
                label: "Duplicate",
                onClick: (webhook: WebhookListItem) => handleDuplicate(webhook),
                className:
                    "text-gray-700 border-gray-300 hover:bg-gray-100 p-2",
                IconComponent: IconCopy,
                iconOnly: true,
            });
        }

        if (data.permissions?.canDelete) {
            actionsList.push({
                label: "Delete",
                onClick: (webhook: WebhookListItem) => handleDelete(webhook),
                className:
                    "text-danger-600 border-danger-600 hover:bg-danger-50 p-2",
                IconComponent: IconTrash,
                iconOnly: true,
            });
        }

        return actionsList;
    });
</script>

<svelte:head>
    <title>Webhooks - {page.params.namespace} - Flowctl</title>
</svelte:head>

<Header
    breadcrumbs={[
        { label: page.params.namespace!, url: `/view/${page.params.namespace}/flows` },
        { label: "Webhooks" },
    ]}
>
    {#snippet children()}
        <div class="mb-10"></div>
    {/snippet}
</Header>

<div class="p-12">
    <PageHeader
        title="Webhooks"
        subtitle="Create reusable webhook destinations for flow notifications."
        actions={
            data.permissions?.canCreate
                ? [
                      {
                          label: "Add Webhook",
                          onClick: handleAdd,
                          variant: "primary",
                          IconComponent: IconPlus,
                          iconSize: 16,
                      },
                  ]
                : []
        }
    />

    <Table
        columns={tableColumns}
        data={webhooks}
        actions={tableActions()}
        loading={loading}
        emptyMessage="No webhooks yet"
        EmptyIconComponent={IconWebhook}
        emptyIconSize={48}
    />
</div>

{#if showModal}
    <WebhookModal
        isEditMode={isEditMode}
        webhookData={selectedWebhook}
        onSave={handleSave}
        onClose={handleModalClose}
        onTest={
            isEditMode && selectedWebhook
                ? () => apiClient.namespaceWebhooks.test(data.namespace, selectedWebhook.id)
                : undefined
        }
    />
{/if}

{#if showDeleteModal && deleteTarget}
    <DeleteModal
        title="Delete Webhook"
        itemName={deleteTarget.name}
        onConfirm={confirmDelete}
        onClose={closeDeleteModal}
    />
{/if}

{#if showDuplicateModal && duplicateTarget}
    <WebhookDuplicateModal
        webhook={duplicateTarget}
        onConfirm={confirmDuplicate}
        onClose={closeDuplicateModal}
    />
{/if}
