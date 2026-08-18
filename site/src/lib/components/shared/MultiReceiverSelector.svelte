<script lang="ts">
    import { apiClient } from "$lib/apiClient";
    import { handleInlineError } from "$lib/utils/errorHandling";
    import OatSelect from "$lib/components/shared/OatSelect.svelte";
    import type { User, Group } from "$lib/types";
    import IconUsers from "@tabler/icons-svelte/icons/users";
    import IconUser from "@tabler/icons-svelte/icons/user";
    import IconX from "@tabler/icons-svelte/icons/x";
    import IconMail from "@tabler/icons-svelte/icons/mail";

    let {
        selectedReceivers = $bindable([]),
        disabled = false,
    }: {
        selectedReceivers: string[];
        disabled?: boolean;
    } = $props();

    let searchQuery = $state("");
    let searchType = $state<"user" | "group">("user");
    let searchResults = $state<(User | Group)[]>([]);
    let loading = $state(false);
    let searchInput: HTMLInputElement;
    let triggerEl: HTMLButtonElement;
    let popoverEl: HTMLDivElement;

    interface SelectedItem {
        type: "user" | "group";
        id: string;
        name: string;
        value: string;
    }

    function isValidEmail(email: string): boolean {
        if (email.length < 3 || email.length > 254) return false;
        return /^[a-zA-Z0-9.!#$%&'*+/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$/.test(email);
    }

    let showAddEmailOption = $derived(
        searchType === "user" && isValidEmail(searchQuery) && !selectedReceivers.includes(searchQuery),
    );

    let selectedItems = $derived.by(() => {
        if (!selectedReceivers || selectedReceivers.length === 0) return [];
        return selectedReceivers.map((r) => {
            if (r.startsWith("group:")) {
                const groupName = r.substring(6);
                return { type: "group" as const, id: groupName, name: groupName, value: r };
            } else {
                return { type: "user" as const, id: r, name: r, value: r };
            }
        });
    });

    async function loadSubjects() {
        loading = true;
        try {
            if (searchType === "user") {
                const response = await apiClient.users.list({ filter: searchQuery, count_per_page: 20 });
                searchResults = response.users || [];
            } else {
                const response = await apiClient.groups.list({ filter: searchQuery, count_per_page: 20 });
                searchResults = response.groups || [];
            }
        } catch (error) {
            handleInlineError(error, searchType === "user" ? "Unable to Load Users" : "Unable to Load Groups");
            searchResults = [];
            closeDropdown();
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
        if (disabled) return;
        try { popoverEl?.showPopover(); } catch {}
        positionPopover();
        loadSubjects();
        setTimeout(() => searchInput?.focus(), 0);
    }

    function closeDropdown() {
        try { popoverEl?.hidePopover(); } catch {}
    }

    let searchTimer: number;
    function handleSearchInput() {
        clearTimeout(searchTimer);
        searchTimer = setTimeout(() => loadSubjects(), 300);
    }

    async function handleTypeChange() {
        searchQuery = "";
        searchResults = [];
        await loadSubjects();
        openDropdown();
    }

    function selectSubject(subject: User | Group) {
        const isUser = "username" in subject;
        const name = isUser ? (subject as User).username : (subject as Group).name;
        const subjectValue = isUser ? name : `group:${name}`;
        if (selectedReceivers.includes(subjectValue)) return;
        selectedReceivers = [...selectedReceivers, subjectValue];
        searchQuery = "";
        closeDropdown();
        searchResults = [];
    }

    function addCustomEmail() {
        if (!isValidEmail(searchQuery) || selectedReceivers.includes(searchQuery)) return;
        selectedReceivers = [...selectedReceivers, searchQuery];
        searchQuery = "";
        closeDropdown();
    }

    function removeReceiver(index: number) {
        selectedReceivers = selectedReceivers.filter((_, i) => i !== index);
    }

    function handleOutsideClick(event: MouseEvent) {
        const target = event.target as HTMLElement;
        if (!target.closest('.multi-receiver-selector') && !target.closest('[popover]')) {
            closeDropdown();
        }
    }

    import { onMount, onDestroy } from 'svelte';

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

<div class="multi-receiver-selector vstack gap-2">
    <div class="hstack gap-2">
        <OatSelect
            bind:value={searchType}
            options={[
                { value: 'user', label: 'User' },
                { value: 'group', label: 'Group' }
            ]}
            onchange={handleTypeChange}
            {disabled}
        />

        <div style="flex: 1">
            <button
                type="button"
                bind:this={triggerEl}
                onclick={openDropdown}
                class="outline small w-100 hstack justify-between"
                {disabled}
            >
                <span class="text-lighter">
                    {searchType === "user" ? "Search users or enter email..." : "Search groups..."}
                </span>
            </button>

            <div bind:this={popoverEl} popover="manual" class="selector-popover">
                <div style="padding: var(--space-2); border-bottom: 1px solid var(--border)">
                    <input
                        type="search"
                        bind:this={searchInput}
                        bind:value={searchQuery}
                        oninput={handleSearchInput}
                        placeholder={searchType === "user" ? "Search users or enter email..." : "Search groups..."}
                        aria-busy={loading}
                        autocomplete="off"
                    />
                </div>
                <div style="max-height: 14rem; overflow-y: auto">
                    {#if showAddEmailOption}
                        <button type="button" class="dropdown-item" onclick={addCustomEmail} style="background: var(--faint)">
                            <div class="icon-box">
                                <IconMail size={16} />
                            </div>
                            <div>
                                <div class="font-medium">Add "{searchQuery}"</div>
                                <div class="text-lighter text-xs">Add external email</div>
                            </div>
                        </button>
                    {/if}
                    {#if searchResults.length > 0}
                        {#each searchResults as subject}
                            <button type="button" class="dropdown-item" onclick={() => selectSubject(subject)}>
                                <div class="icon-box">
                                    {#if searchType === "user"}
                                        <IconUser size={16} />
                                    {:else}
                                        <IconUsers size={16} />
                                    {/if}
                                </div>
                                <div>
                                    <div class="font-medium">{"name" in subject ? subject.name : subject.username}</div>
                                    <div class="text-lighter text-xs">{subject.id}</div>
                                </div>
                            </button>
                        {/each}
                    {:else if !loading && !showAddEmailOption}
                        <div class="dropdown-empty">
                            {searchType === "user" ? "No users found. Enter a valid email to add." : "No groups found"}
                        </div>
                    {/if}
                </div>
            </div>
        </div>
    </div>

    {#if selectedItems.length > 0}
        <div class="hstack" style="flex-wrap: wrap; gap: 0.5rem">
            {#each selectedItems as item, index (item.value)}
                <span class="badge primary hstack gap-1">
                    {#if item.type === "user"}
                        <IconUser size={12} />
                    {:else}
                        <IconUsers size={12} />
                    {/if}
                    {item.name}
                    <button type="button" onclick={() => removeReceiver(index)} {disabled} aria-label="Remove {item.name}" style="all: unset; cursor: pointer; display: flex">
                        <IconX size={12} />
                    </button>
                </span>
            {/each}
        </div>
    {:else}
        <div class="text-lighter align-center text-sm">No receivers selected</div>
    {/if}
</div>
