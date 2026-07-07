<script lang="ts">
    import { apiClient } from "$lib/apiClient";
    import type { ApprovalDetailsResp } from "$lib/types";
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
    let dialogEl: HTMLDialogElement;

    $effect(() => {
        if (open && approvalId) {
            fetchApprovalDetails();
        }
    });

    $effect(() => {
        if (open && dialogEl) {
            dialogEl.showModal();
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
        dialogEl?.close();
    }

    async function handleApprove() {
        if (!approval) return;
        actionLoading = true;
        try {
            await onApprove(approval.id);
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
            await fetchApprovalDetails();
        } catch (err) {
            handleInlineError(err, "Unable to Reject Request");
        } finally {
            actionLoading = false;
        }
    }
</script>

{#if open}
    <dialog bind:this={dialogEl} onclose={() => { open = false; approval = null; error = null; }}>
        <header>
            <div class="hstack gap-2">
                <div class="icon-box">
                    <svg class="icon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                    </svg>
                </div>
                <h3>Approval Details</h3>
            </div>
        </header>

        <section>
            {#if loading}
                <div class="vstack items-center gap-2 centered-msg" aria-busy="true">Loading...</div>
            {:else if error}
                <div class="vstack items-center gap-2 centered-msg">
                    <div class="error-text">{error}</div>
                    <button data-variant="secondary" onclick={fetchApprovalDetails}>
                        Try again
                    </button>
                </div>
            {:else if approval}
                <div class="vstack gap-4">
                    <!-- Approval Overview -->
                    <div class="info-box">
                        <div class="info-grid">
                            <div>
                                <span class="info-label text-sm font-medium">Action ID</span>
                                <span class="text-sm">{approval.action_id}</span>
                            </div>
                            <div>
                                <span class="info-label text-sm font-medium">Status</span>
                                <span class="text-sm capitalize">{approval.status}</span>
                            </div>
                            <div>
                                <span class="info-label text-sm font-medium">Execution ID</span>
                                <a href="/view/{encodeURIComponent(namespace)}/results/{approval.flow_id}/{approval.exec_id}">
                                    <span class="font-mono text-sm">{approval.exec_id}</span>
                                </a>
                            </div>
                            <div>
                                <span class="info-label text-sm font-medium">Requested By</span>
                                <span class="text-sm">{approval.requested_by}</span>
                            </div>
                            <div>
                                <span class="info-label text-sm font-medium">Reviewer</span>
                                <span class="text-sm">{approval.approved_by || "N/A"}</span>
                            </div>
                            <div>
                                <span class="info-label text-sm font-medium">Flow</span>
                                <span class="text-sm">{approval.flow_name}</span>
                            </div>
                            <div>
                                <span class="info-label text-sm font-medium">Created</span>
                                <span class="text-sm">{formatDateTime(approval.created_at, "Never")}</span>
                            </div>
                        </div>

                        {#if (approval.approvers && approval.approvers.length > 0) || (approval.approval_groups && approval.approval_groups.length > 0)}
                            <div class="approvers-section">
                                <span class="info-label text-sm font-medium">Restricted to</span>
                                <div class="tags-row">
                                    {#if approval.approvers}
                                        {#each approval.approvers as approver}
                                            <span class="tag tag-user" title="User approver">👤 {approver}</span>
                                        {/each}
                                    {/if}
                                    {#if approval.approval_groups}
                                        {#each approval.approval_groups as group}
                                            <span class="tag tag-group" title="Group approver">🏷 {group}</span>
                                        {/each}
                                    {/if}
                                </div>
                            </div>
                        {:else}
                            <div class="approvers-section">
                                <span class="info-label text-sm font-medium">Restricted to</span>
                                <span class="text-sm text-lighter">Any namespace reviewer or admin</span>
                            </div>
                        {/if}
                    </div>

                    <!-- Execution Inputs -->
                    {#if approval.inputs}
                        <details>
                            <summary>Inputs</summary>
                            <pre><code>{JSON.stringify(approval.inputs, null, 2)}</code></pre>
                        </details>
                    {/if}
                </div>
            {/if}
        </section>

        {#if approval && approval.status === "pending"}
            <footer>
                <button
                    data-variant="secondary"
                    onclick={handleReject}
                    disabled={actionLoading}
                >
                    <span class="hstack gap-2 justify-center" aria-busy={actionLoading} data-spinner="small">
                        Reject
                    </span>
                </button>
                <button
                    onclick={handleApprove}
                    disabled={actionLoading}
                >
                    <span class="hstack gap-2 justify-center" aria-busy={actionLoading} data-spinner="small">
                        Approve
                    </span>
                </button>
            </footer>
        {:else}
            <footer>
                <button
                    data-variant="secondary"
                    onclick={closeModal}
                    use:autofocus
                >
                    Close
                </button>
            </footer>
        {/if}
    </dialog>
{/if}

<style>
    dialog {
        max-width: 56rem;
        width: 100%;
    }
    .centered-msg {
        padding: 2rem;
    }
    .error-text {
        color: var(--danger);
    }
    .info-box {
        background: var(--faint);
        border-radius: 0.5rem;
        padding: 1rem;
    }
    .info-grid {
        display: grid;
        grid-template-columns: 1fr 1fr;
        gap: 1rem;
    }
    @media (max-width: 768px) {
        .info-grid {
            grid-template-columns: 1fr;
        }
    }
    .info-label {
        display: block;
        margin-bottom: 0.25rem;
    }
    .capitalize {
        text-transform: capitalize;
    }
    .approvers-section {
        margin-top: 0.75rem;
        padding-top: 0.75rem;
        border-top: 1px solid var(--border);
    }
    .tags-row {
        display: flex;
        flex-wrap: wrap;
        gap: 0.375rem;
        margin-top: 0.375rem;
    }
    .tag {
        display: inline-flex;
        align-items: center;
        gap: 0.25rem;
        padding: 0.2rem 0.6rem;
        border-radius: 9999px;
        font-size: 0.75rem;
        font-weight: 500;
    }
    .tag-user {
        background: color-mix(in srgb, var(--primary) 12%, transparent);
        color: var(--primary);
        border: 1px solid color-mix(in srgb, var(--primary) 25%, transparent);
    }
    .tag-group {
        background: color-mix(in srgb, var(--warning, #f59e0b) 12%, transparent);
        color: var(--warning, #f59e0b);
        border: 1px solid color-mix(in srgb, var(--warning, #f59e0b) 25%, transparent);
    }
    .text-lighter {
        color: var(--muted-foreground);
    }
</style>
