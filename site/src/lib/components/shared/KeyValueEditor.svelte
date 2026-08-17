<script lang="ts">
    function parseInitial(json: string | undefined): Array<{ name: string; value: string }> {
        if (!json) return [{ name: "", value: "" }];
        try {
            const obj = JSON.parse(json);
            if (typeof obj === "object" && obj !== null) {
                const entries = Object.entries(obj as Record<string, unknown>);
                if (entries.length > 0) {
                    return entries.map(([k, v]) => ({ name: k, value: String(v) }));
                }
            }
        } catch {
            // ignore
        }
        return [{ name: "", value: "" }];
    }

    let {
        pairs = $bindable(parseInitial(initialValue)),
        initialValue,
        onchange,
        keyPlaceholder = "KEY",
        valuePlaceholder = "value",
        disabled = false,
    }: {
        pairs?: Array<{ name: string; value: string }>;
        initialValue?: string;
        onchange?: (json: string) => void;
        keyPlaceholder?: string;
        valuePlaceholder?: string;
        disabled?: boolean;
    } = $props();

    function serialize() {
        const obj: Record<string, string> = {};
        for (const pair of pairs) {
            if (pair.name.trim() !== "") {
                obj[pair.name.trim()] = pair.value;
            }
        }
        return JSON.stringify(obj);
    }

    function notifyChange() {
        onchange?.(serialize());
    }

    function addPair() {
        pairs.push({ name: "", value: "" });
        notifyChange();
    }

    function removePair(i: number) {
        pairs.splice(i, 1);
        if (pairs.length === 0) {
            pairs.push({ name: "", value: "" });
        }
        notifyChange();
    }

    function handleKeyInput(e: Event, i: number) {
        pairs[i].name = (e.currentTarget as HTMLInputElement).value;
        notifyChange();
    }

    function handleValueInput(e: Event, i: number) {
        pairs[i].value = (e.currentTarget as HTMLInputElement).value;
        notifyChange();
    }
</script>

<div class="vstack gap-2">
    {#each pairs as pair, i}
        <div class="hstack gap-2">
            <input
                type="text"
                value={pair.name}
                oninput={(e) => handleKeyInput(e, i)}
                placeholder={keyPlaceholder}
                class="kv-input font-mono"
                {disabled}
            />
            <span class="text-lighter">=</span>
            <input
                type="text"
                value={pair.value}
                oninput={(e) => handleValueInput(e, i)}
                placeholder={valuePlaceholder}
                class="kv-input font-mono"
                {disabled}
            />
            {#if !disabled && (pairs.length > 1 || pair.name !== "" || pair.value !== "")}
                <button
                    onclick={() => removePair(i)}
                    type="button"
                    data-variant="danger"
                    class="remove-btn"
                    aria-label="Remove pair"
                >
                    <svg class="icon-sm" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                    </svg>
                </button>
            {:else}
                <div class="icon-sm"></div>
            {/if}
        </div>
    {/each}
    {#if !disabled}
        <button
            onclick={addPair}
            type="button"
            data-variant="secondary"
            class="add-btn"
        >
            + Add
        </button>
    {/if}
</div>

<style>
    .kv-input {
        flex: 1;
    }

    .remove-btn {
        padding: 0.25rem;
        background: none;
        border: none;
        color: var(--muted-foreground);
        cursor: pointer;
    }

    .remove-btn:hover {
        color: var(--danger);
    }

    .icon-sm {
        width: 1rem;
        height: 1rem;
    }

    .add-btn {
        font-size: 0.75rem;
        align-self: flex-start;
        padding: 0.25rem 0.5rem;
        background: none;
        border: none;
        color: var(--primary);
        cursor: pointer;
    }

    .add-btn:hover {
        text-decoration: underline;
    }
</style>
