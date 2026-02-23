<script lang="ts">
    import { apiClient } from "$lib/apiClient";
    import type { ApprovalDetailsResp } from "$lib/types";
    import JsonDisplay from "$lib/components/shared/JsonDisplay.svelte";
    import { handleInlineError } from "$lib/utils/errorHandling";
    import { autofocus } from "$lib/utils/autofocus";
    import { formatDateTime } from "$lib/utils";

    let {
        open = $bindable(),
        approvalId,
        namespace,
        onApprove,
        onReject,
    }: {
        open: boolean;
        approvalId: string;
        namespace: string;
        onApprove: (approvalId: string) => Promise<void>;
        onReject: (approvalId: string) => Promise<void>;
    } = $props();

    let approval: ApprovalDetailsResp | null = $state(null);
    let loading = $state(false);
    let error = $state<string | null>(null);
    let actionLoading = $state(false);

    // Fetch approval details when modal opens
    $effect(() => {
        if (open && approvalId) {
            fetchApprovalDetails();
        }
    });

    async function fetchApprovalDetails() {
        loading = true;
        error = null;
        try {
            approval = await apiClient.approvals.get(namespace, approvalId);
        } catch (err) {
            error = "Failed to load approval details";
            handleInlineError(err, "Unable to Load Approval Details");
        } finally {
            loading = false;
        }
    }

    function closeModal() {
        open = false;
        approval = null;
        error = null;
    }

    function handleBackdropClick(event: MouseEvent) {
        if (event.target === event.currentTarget) {
            closeModal();
        }
    }

    function handleKeydown(event: KeyboardEvent) {
        if (event.key === "Escape") {
            closeModal();
        }
    }

    async function handleApprove() {
        if (!approval) return;
        actionLoading = true;
        try {
            await onApprove(approval.id);
            // Refresh the approval data after action
            await fetchApprovalDetails();
        } catch (err) {
            handleInlineError(err, "Unable to Approve Request");
        } finally {
            actionLoading = false;
        }
    }

    async function handleReject() {
        if (!approval) return;
        actionLoading = true;
        try {
            await onReject(approval.id);
            // Refresh the approval data after action
            await fetchApprovalDetails();
        } catch (err) {
            handleInlineError(err, "Unable to Reject Request");
        } finally {
            actionLoading = false;
        }
    }

</script>

{#if open}
    <!-- Modal backdrop -->
    <div
        class="fixed inset-0 z-50 flex items-center justify-center bg-overlay p-4"
        onclick={handleBackdropClick}
        onkeydown={(e) => e.key === "Escape" && closeModal()}
        role="dialog"
        aria-modal="true"
        tabindex="-1"
    >
        <!-- Modal content -->
        <div
            class="bg-card rounded-lg shadow-xl w-full max-w-4xl max-h-[90vh] overflow-hidden"
        >
            <!-- Modal header -->
            <div
                class="px-6 py-4 border-b border-border flex items-center justify-between"
            >
                <div class="flex items-center space-x-3">
                    <div
                        class="w-8 h-8 bg-primary-100 rounded-lg flex items-center justify-center"
                    >
                        <svg
                            class="w-5 h-5 text-primary-600"
                            fill="none"
                            stroke="currentColor"
                            viewBox="0 0 24 24"
                        >
                            <path
                                stroke-linecap="round"
                                stroke-linejoin="round"
                                stroke-width="2"
                                d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
                            />
                        </svg>
                    </div>
                    <h3 class="text-lg font-semibold text-foreground">
                        Approval Details
                    </h3>
                </div>
                <button
                    onclick={closeModal}
                    class="text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
                    aria-label="Close modal"
                    use:autofocus
                >
                    <svg
                        class="w-6 h-6"
                        fill="none"
                        stroke="currentColor"
                        viewBox="0 0 24 24"
                    >
                        <path
                            stroke-linecap="round"
                            stroke-linejoin="round"
                            stroke-width="2"
                            d="M6 18L18 6M6 6l12 12"
                        />
                    </svg>
                </button>
            </div>

            <!-- Modal body -->
            <div class="px-6 py-4 overflow-y-auto max-h-[calc(90vh-140px)]">
                {#if loading}
                    <div class="flex items-center justify-center py-8">
                        <div
                            class="animate-spin rounded-full h-8 w-8 border-b-2 border-primary-600"
                        ></div>
                    </div>
                {:else if error}
                    <div class="text-center py-8">
                        <div class="text-danger-600 mb-2">{error}</div>
                        <button
                            onclick={fetchApprovalDetails}
                            class="text-primary-600 hover:text-primary-800 underline cursor-pointer"
                        >
                            Try again
                        </button>
                    </div>
                {:else if approval}
                    <div class="space-y-6">
                        <!-- Approval Overview -->
                        <div class="bg-muted rounded-lg p-4">
                            <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                                <div>
                                    <span
                                        class="block text-sm font-medium text-foreground mb-1"
                                        >Action ID</span
                                    >
                                    <span class="text-sm text-foreground"
                                        >{approval.action_id}</span
                                    >
                                </div>
                                <div>
                                    <span
                                        class="block text-sm font-medium text-foreground mb-1"
                                        >Status</span
                                    >
                                    <span
                                        class="text-sm text-foreground capitalize"
                                        >{approval.status}</span
                                    >
                                </div>
                                <div>
                                    <span class="block text-sm font-medium text-foreground mb-1">Execution ID</span>
                                    <a href="/view/{namespace}/results/{approval.flow_id}/{approval.exec_id}">
                                        <span class="font-mono hover:underline text-sm text-link hover:text-link-hover">{approval.exec_id}</span>
                                    </a>
                                </div>
                                <div>
                                    <span
                                        class="block text-sm font-medium text-foreground mb-1"
                                        >Requested By</span
                                    >
                                    <span class="text-sm text-foreground"
                                        >{approval.requested_by}</span
                                    >
                                </div>
                                <div>
                                    <span
                                        class="block text-sm font-medium text-foreground mb-1"
                                        >Reviewer</span
                                    >
                                    <span class="text-sm text-foreground"
                                        >{approval.approved_by || "N/A"}</span
                                    >
                                </div>
                                <div>
                                    <span
                                        class="block text-sm font-medium text-foreground mb-1"
                                        >Flow</span
                                    >
                                    <span class="text-sm text-foreground"
                                        >{approval.flow_name}</span
                                    >
                                </div>
                                <div>
                                    <span
                                        class="block text-sm font-medium text-foreground mb-1"
                                        >Created</span
                                    >
                                    <span class="text-sm text-foreground"
                                        >{formatDateTime(approval.created_at, "Never")}</span
                                    >
                                </div>
                            </div>
                        </div>

                        <!-- Execution Inputs -->
                        {#if approval.inputs}
                            <div>
                                <h4
                                    class="text-base font-semibold text-foreground mb-3"
                                >
                                    Execution Inputs
                                </h4>
                                <JsonDisplay data={approval.inputs} />
                            </div>
                        {/if}

                        <!-- Previous Action Logs -->
                        {#if approval.previous_logs}
                            <div>
                                <h4 class="text-base font-semibold text-foreground mb-3">
                                    Output de acciones anteriores
                                </h4>
                                <div class="bg-black rounded-lg p-4 overflow-x-auto max-h-96 overflow-y-auto">
                                    <pre class="text-green-400 text-xs font-mono whitespace-pre-wrap">{approval.previous_logs}</pre>
                                </div>
                            </div>
                        {/if}

                        <!-- Action Buttons -->
                        {#if approval && approval.status === "pending"}
                            <div
                                class="flex justify-end gap-3 pt-6 border-t border-border"
                            >
                                <button
                                    onclick={handleReject}
                                    disabled={actionLoading}
                                    class="px-4 py-2 text-sm font-medium text-foreground bg-subtle border border-transparent rounded-lg hover:bg-subtle-hover focus:outline-none focus:border-transparent disabled:opacity-50 cursor-pointer"
                                >
                                    {#if actionLoading}
                                        <svg
                                            class="animate-spin -ml-1 mr-2 h-4 w-4 text-danger-900 inline"
                                            xmlns="http://www.w3.org/2000/svg"
                                            fill="none"
                                            viewBox="0 0 24 24"
                                        >
                                            <circle
                                                class="opacity-25"
                                                cx="12"
                                                cy="12"
                                                r="10"
                                                stroke="currentColor"
                                                stroke-width="4"
                                            ></circle>
                                            <path
                                                class="opacity-75"
                                                fill="currentColor"
                                                d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                                            ></path>
                                        </svg>
                                    {/if}
                                    Reject
                                </button>
                                <button
                                    onclick={handleApprove}
                                    disabled={actionLoading}
                                    class="px-4 py-2 text-sm font-medium text-white bg-primary-500 border border-transparent rounded-lg hover:bg-primary-600 focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent disabled:opacity-50 cursor-pointer"
                                >
                                    {#if actionLoading}
                                        <svg
                                            class="animate-spin -ml-1 mr-2 h-4 w-4 text-success-900 inline"
                                            xmlns="http://www.w3.org/2000/svg"
                                            fill="none"
                                            viewBox="0 0 24 24"
                                        >
                                            <circle
                                                class="opacity-25"
                                                cx="12"
                                                cy="12"
                                                r="10"
                                                stroke="currentColor"
                                                stroke-width="4"
                                            ></circle>
                                            <path
                                                class="opacity-75"
                                                fill="currentColor"
                                                d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                                            ></path>
                                        </svg>
                                    {/if}
                                    Approve
                                </button>
                            </div>
                        {/if}
                    </div>
                {/if}
            </div>
        </div>
    </div>
{/if}
