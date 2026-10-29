<script lang="ts">
    import { handleInlineError } from "$lib/utils/errorHandling";
    import OatSelect from "$lib/components/shared/OatSelect.svelte";
    import type { CredentialResp, NodeImportResp } from "$lib/types";

    interface Props {
        credentials: CredentialResp[];
        onImport: (file: File, credentialId: string) => Promise<NodeImportResp>;
        onClose: () => void;
    }

    let { credentials, onImport, onClose }: Props = $props();

    let selectedCredentialId = $state("");
    let selectedFile = $state<File | null>(null);
    let loading = $state(false);
    let result = $state<NodeImportResp | null>(null);
    let dialogEl: HTMLDialogElement;

    $effect(() => {
        if (dialogEl) {
            dialogEl.showModal();
        }
    });

    function onFileChange(e: Event) {
        const input = e.currentTarget as HTMLInputElement;
        selectedFile = input.files?.[0] ?? null;
    }

    function downloadTemplate() {
        const csvContent =
            "name,hostname,port,username,connection_type,tags\n" +
            "mynode,192.168.1.1,22,admin,ssh,\"production,web\"\n";
        const blob = new Blob([csvContent], { type: "text/csv" });
        const url = URL.createObjectURL(blob);
        const a = document.createElement("a");
        a.href = url;
        a.download = "nodes_template.csv";
        a.click();
        URL.revokeObjectURL(url);
    }

    async function handleSubmit() {
        if (!selectedFile || !selectedCredentialId) return;
        loading = true;
        result = null;
        try {
            result = await onImport(selectedFile, selectedCredentialId);
        } catch (err) {
            handleInlineError(err, "Failed to import nodes");
        } finally {
            loading = false;
        }
    }
</script>

<dialog bind:this={dialogEl} onclose={onClose}>
    <form onsubmit={(e) => { e.preventDefault(); handleSubmit(); }}>
        <header>
            <h3>Import Nodes from CSV</h3>
        </header>

        <section>
            {#if !result}
                <div data-field>
                    <label>CSV File</label>
                    <input
                        type="file"
                        accept=".csv,text/csv"
                        onchange={onFileChange}
                        required
                        disabled={loading}
                    />
                    <p class="text-xs text-light mt-1">
                        Required columns: <code>name, hostname, port, username, connection_type, tags</code>
                    </p>
                </div>

                <div data-field>
                    <label>Credential</label>
                    <OatSelect
                        bind:value={selectedCredentialId}
                        options={credentials.map(c => ({ value: c.id, label: `${c.name} (${c.key_type})` }))}
                        placeholder="Select credential for all imported nodes"
                        required
                        disabled={loading}
                    />
                </div>

                <div class="mt-2">
                    <button
                        type="button"
                        data-variant="secondary"
                        class="small"
                        onclick={downloadTemplate}
                    >
                        Download CSV Template
                    </button>
                </div>
            {:else}
                <div class="vstack gap-3">
                    <div class="hstack gap-2 items-center">
                        <span class="text-sm font-medium">{result.imported} node{result.imported === 1 ? '' : 's'} imported successfully</span>
                    </div>

                    {#if result.errors && result.errors.length > 0}
                        <div class="vstack gap-1">
                            <p class="text-sm font-medium text-danger-600">{result.errors.length} row{result.errors.length === 1 ? '' : 's'} failed:</p>
                            <div class="import-errors">
                                {#each result.errors as err}
                                    <div class="import-error-row">
                                        <span class="text-xs text-light">Row {err.row}{err.name ? ` (${err.name})` : ''}:</span>
                                        <span class="text-xs text-danger-600">{err.error}</span>
                                    </div>
                                {/each}
                            </div>
                        </div>
                    {/if}
                </div>
            {/if}
        </section>

        <footer>
            <button
                type="button"
                data-variant="secondary"
                onclick={onClose}
                disabled={loading}
            >
                {result ? 'Close' : 'Cancel'}
            </button>
            {#if !result}
                <button
                    type="submit"
                    disabled={loading || !selectedFile || !selectedCredentialId}
                >
                    <span class="hstack gap-2 justify-center" aria-busy={loading} data-spinner="small">
                        Import
                    </span>
                </button>
            {/if}
        </footer>
    </form>
</dialog>

<style>
    dialog {
        max-width: 32rem;
        width: 100%;
    }

    .import-errors {
        max-height: 12rem;
        overflow-y: auto;
        border: 1px solid var(--border);
        border-radius: var(--radius);
        padding: 0.5rem;
        display: flex;
        flex-direction: column;
        gap: 0.25rem;
    }

    .import-error-row {
        display: flex;
        flex-direction: column;
        gap: 0.125rem;
        padding: 0.25rem 0;
        border-bottom: 1px solid var(--border);
    }

    .import-error-row:last-child {
        border-bottom: none;
    }
</style>
