<script lang="ts">
    import { apiClient } from "$lib/apiClient";
    import { handleInlineError } from "$lib/utils/errorHandling";
    import type { FlowGroupResp } from "$lib/types";
    import IconFolder from "@tabler/icons-svelte/icons/folder";
    import IconX from "@tabler/icons-svelte/icons/x";
    import IconPlus from "@tabler/icons-svelte/icons/plus";

    let {
        namespace,
        value = $bindable(""),
        allowCreate = true,
    }: {
        namespace: string;
        value: string;
        allowCreate?: boolean;
    } = $props();

    let searchQuery = $state("");
    let groups = $state<FlowGroupResp[]>([]);
    let loading = $state(false);
    let searchInput: HTMLInputElement;
    let triggerEl: HTMLButtonElement;
    let popoverEl: HTMLDivElement;

    let filteredGroups = $derived(
        groups.filter((g) => g.prefix.toLowerCase().includes(searchQuery.toLowerCase())),
    );

    let showCreateOption = $derived(
        allowCreate &&
            searchQuery.trim() !== "" &&
            !groups.some((g) => g.prefix.toLowerCase() === searchQuery.toLowerCase()),
    );

    async function loadGroups() {
        loading = true;
        try {
            const result = await apiClient.flows.groups.list(namespace);
            groups = result.groups || [];
        } catch (error) {
            handleInlineError(error, "Unable to load flow groups");
            groups = [];
        } finally {
            loading = false;
        }
    }

    function positionPopover() {
        if (!triggerEl || !popoverEl) return;
        const r = triggerEl.getBoundingClientRect();
        popoverEl.style.top = `${r.bottom + 4}px`;
        popoverEl.style.left = `${r.left}px`;
        popoverEl.style.width = `${r.width}px`;
    }

    function openDropdown() {
        try { popoverEl?.showPopover(); } catch {}
        positionPopover();
        loadGroups();
        setTimeout(() => searchInput?.focus(), 0);
    }

    function closeDropdown() {
        try { popoverEl?.hidePopover(); } catch {}
    }

    function selectGroup(name: string) {
        value = name;
        searchQuery = "";
        closeDropdown();
    }

    function clear() {
        value = "";
        searchQuery = "";
    }

    function handleOutsideClick(event: MouseEvent) {
        const target = event.target as HTMLElement;
        if (!target.closest('.flow-group-selector') && !target.closest('[popover]')) {
            closeDropdown();
        }
    }

    import { onMount, onDestroy } from "svelte";

    let scrollCleanup: (() => void) | null = null;

    onMount(() => {
        const onScroll = () => {
            if (popoverEl?.matches(':popover-open')) closeDropdown();
        };
        document.addEventListener('scroll', onScroll, { capture: true, passive: true });
        scrollCleanup = () => document.removeEventListener('scroll', onScroll, { capture: true });
    });

    onDestroy(() => scrollCleanup?.());
</script>

<svelte:window on:click={handleOutsideClick} />

<div class="flow-group-selector vstack" style="gap: 0.25rem">
    <label for="flow-group">
        Group
        <span class="text-lighter" style="font-weight: normal; font-size: 0.875rem">(optional)</span>
    </label>

    {#if value}
        <div class="hstack gap-2" style="padding: 0.5rem 0.75rem; background: var(--card); border: 1px solid var(--border); border-radius: 0.375rem">
            <IconFolder size={16} class="text-lighter" />
            <span style="flex: 1; font-size: 0.875rem">{value}</span>
            <button type="button" class="ghost icon small" onclick={clear} style="padding: 0.125rem">
                <IconX size={16} />
            </button>
        </div>
    {:else}
        <div>
            <button
                type="button"
                bind:this={triggerEl}
                onclick={openDropdown}
                class="outline small w-100 hstack justify-between"
            >
                <span class="text-lighter">
                    {allowCreate ? "Search or create a group..." : "Search groups..."}
                </span>
            </button>

            <div bind:this={popoverEl} popover="manual" class="selector-popover">
                <div style="padding: var(--space-2); border-bottom: 1px solid var(--border)">
                    <input
                        type="search"
                        bind:this={searchInput}
                        bind:value={searchQuery}
                        placeholder={allowCreate ? "Search or create a group..." : "Search groups..."}
                        aria-busy={loading}
                        autocomplete="off"
                    />
                </div>
                <div style="max-height: 14rem; overflow-y: auto">
                    {#if showCreateOption}
                        <button type="button" class="dropdown-item" onclick={() => selectGroup(searchQuery.trim())}>
                            <IconPlus size={16} style="color: var(--primary)" />
                            <span>Create "{searchQuery.trim()}"</span>
                        </button>
                    {/if}
                    {#if filteredGroups.length > 0}
                        {#each filteredGroups as group}
                            <button type="button" class="dropdown-item" onclick={() => selectGroup(group.prefix)}>
                                <IconFolder size={16} class="text-lighter" />
                                <div>
                                    <div class="font-medium text-sm">{group.prefix}</div>
                                    {#if group.flow_count > 0}
                                        <div class="text-lighter text-xs">
                                            {group.flow_count} flow{group.flow_count !== 1 ? "s" : ""}
                                        </div>
                                    {/if}
                                </div>
                            </button>
                        {/each}
                    {:else if !loading && !showCreateOption}
                        <div class="dropdown-empty">
                            {allowCreate ? "No groups found. Type to create one." : "No groups found."}
                        </div>
                    {/if}
                </div>
            </div>
        </div>
    {/if}

    <p class="text-lighter" style="font-size: 0.75rem; margin-top: 0.25rem">
        Assign this flow to a group for organization. Groups are created automatically.
    </p>
</div>
