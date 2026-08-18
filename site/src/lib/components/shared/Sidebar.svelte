<script lang="ts">
    import { onMount } from "svelte";
    import { page } from "$app/state";
    import { apiClient } from "$lib/apiClient";
    import { handleInlineError } from "$lib/utils/errorHandling";
    import { currentUser } from "$lib/stores/auth";
    import { selectedNamespace } from "$lib/stores/namespace";
    import type { Namespace } from "$lib/types";
    import { DEFAULT_PAGE_SIZE } from "$lib/constants";
    import { setContext } from "svelte";
    import {
        permissionChecker,
        clearPermissionCache,
        type ResourcePermissions,
    } from "$lib/utils/permissions";
    import IconChevronDown from "@tabler/icons-svelte/icons/chevron-down";
    import IconGridDots from "@tabler/icons-svelte/icons/grid-dots";
    import IconServer from "@tabler/icons-svelte/icons/server";
    import IconKey from "@tabler/icons-svelte/icons/key";
    import IconLock from "@tabler/icons-svelte/icons/lock";
    import IconUsers from "@tabler/icons-svelte/icons/users";
    import IconCircleCheck from "@tabler/icons-svelte/icons/circle-check";
    import IconClock from "@tabler/icons-svelte/icons/clock";
    import IconSettings from "@tabler/icons-svelte/icons/settings";
    import IconChevronsLeft from "@tabler/icons-svelte/icons/chevrons-left";
    import IconChevronsRight from "@tabler/icons-svelte/icons/chevrons-right";
    import UserDropdown from "./UserDropdown.svelte";
    import ThemeToggle from "./ThemeToggle.svelte";
    import Logo from "./Logo.svelte";
    import { appInfo } from "$lib/stores/auth";

    let { namespace }: { namespace: string } = $props();

    let isCollapsed = $state(false);
    let namespaces = $state<Namespace[]>([]);
    let searchQuery = $state("");
    let searchResults = $state<Namespace[]>([]);
    let currentNamespace = $state(page.params.namespace || namespace);
    let currentNamespaceId = $state<string>("");
    let searchLoading = $state(false);
    let nsSearchInput: HTMLInputElement;
    let permissions = $state<{
        flows: ResourcePermissions;
        nodes: ResourcePermissions;
        credentials: ResourcePermissions;
        secrets: ResourcePermissions;
        members: ResourcePermissions;
        approvals: ResourcePermissions;
        history: ResourcePermissions;
    }>({
        flows: { canCreate: false, canUpdate: false, canDelete: false, canRead: false },
        nodes: { canCreate: false, canUpdate: false, canDelete: false, canRead: false },
        credentials: { canCreate: false, canUpdate: false, canDelete: false, canRead: false },
        secrets: { canCreate: false, canUpdate: false, canDelete: false, canRead: false },
        members: { canCreate: false, canUpdate: false, canDelete: false, canRead: false },
        approvals: { canCreate: false, canUpdate: false, canDelete: false, canRead: false },
        history: { canCreate: false, canUpdate: false, canDelete: false, canRead: false },
    });

    const isActiveLink = (section: string): boolean => {
        const currentPath = page.url.pathname;
        return currentPath.includes(`/${section}`);
    };

    const fetchNamespaces = async (filter = "") => {
        try {
            searchLoading = true;
            const data = await apiClient.namespaces.list({
                count_per_page: DEFAULT_PAGE_SIZE,
                filter: filter,
            });
            const results = data.namespaces || [];

            if (filter) {
                searchResults = results;
            } else {
                namespaces = results;
                searchResults = results;

                const currentNs = namespaces.find((ns) => ns.name === namespace);
                if (currentNs) {
                    currentNamespaceId = currentNs.id;
                } else if (namespaces.length > 0) {
                    currentNamespaceId = namespaces[0].id;
                }
            }
        } catch (error) {
            handleInlineError(error, "Unable to Load Namespaces");
            if (filter) {
                searchResults = [];
            } else {
                namespaces = [];
                searchResults = [];
            }
        } finally {
            searchLoading = false;
        }
    };

    let searchTimer: number;
    const handleSearchInput = () => {
        clearTimeout(searchTimer);
        searchTimer = setTimeout(async () => {
            if (searchQuery.trim()) {
                await fetchNamespaces(searchQuery);
            } else {
                searchResults = namespaces;
            }
        }, 300);
    };

    const handleNsToggle = (e: ToggleEvent) => {
        if (e.newState === "open") {
            searchQuery = "";
            searchResults = namespaces;
            setTimeout(() => nsSearchInput?.focus(), 0);
        }
    };

    const checkAllPermissions = async () => {
        if (!$currentUser || !currentNamespaceId) return;

        const resourceMappings: Record<string, { resource: string; prefix?: string }> = {
            flows: { resource: "flow", prefix: "_" },
            nodes: { resource: "node" },
            credentials: { resource: "credential" },
            secrets: { resource: "namespace_secret" },
            members: { resource: "member" },
            approvals: { resource: "approval" },
            history: { resource: "execution" },
        };

        try {
            const permissionPromises = Object.entries(resourceMappings).map(
                async ([frontendKey, { resource: backendResource, prefix }]) => {
                    const perms = await permissionChecker(
                        $currentUser,
                        backendResource,
                        currentNamespaceId,
                        ["view"],
                        prefix,
                    );
                    return { frontendKey, perms };
                },
            );

            const results = await Promise.all(permissionPromises);
            results.forEach(({ frontendKey, perms }) => {
                permissions[frontendKey as keyof typeof permissions] = perms;
            });
        } catch (error) {
            handleInlineError(error, "Unable to Check Sidebar Permissions");
        }
    };

    const selectNamespace = (ns: Namespace) => {
        searchQuery = "";
        document.getElementById("ns-select")?.hidePopover();
        if (ns.name === namespace) return;
        clearPermissionCache();
        selectedNamespace.set(ns.name);
        window.location.href = `/view/${encodeURIComponent(ns.name)}/flows`;
    };

    const toggleCollapse = () => {
        isCollapsed = !isCollapsed;
    };

    setContext("namespace", namespace);

    onMount(() => {
        fetchNamespaces();
        checkAllPermissions();
    });

    $effect(() => {
        currentNamespace = page.params.namespace || namespace;
        if (currentNamespace) {
            selectedNamespace.set(currentNamespace);
        }
    });

    $effect(() => {
        if ($currentUser && currentNamespaceId) {
            checkAllPermissions();
        }
    });
</script>

<aside data-sidebar aria-label="Main navigation" class:collapsed={isCollapsed}>
    <header>
        <a href="/" class="unstyled vstack items-center gap-1">
            {#if isCollapsed}
                <Logo height="1.5rem" iconOnly={true} />
            {:else}
                <Logo height="2rem" />
                {#if $appInfo}
                    <span class="text-lighter" style="font-size: var(--text-8)">{$appInfo.version}-{$appInfo.commit}</span>
                {/if}
            {/if}
        </a>

        {#if isCollapsed}
            <!-- Collapsed namespace indicator -->
            <div class="ns-collapsed-indicator mt-4" title={currentNamespace || "Select namespace"}>
                <span class="ns-initial">{(currentNamespace || "?").charAt(0).toUpperCase()}</span>
            </div>
        {:else}
            <!-- Namespace Dropdown -->
            <ot-dropdown class="mt-4" style="display: block">
                <button popovertarget="ns-select" class="outline small w-100 hstack justify-between">
                    <span>{currentNamespace || "Select namespace"}</span>
                    <IconChevronDown size={14} />
                </button>
                <div popover id="ns-select" ontoggle={handleNsToggle}>
                    <div style="padding: var(--space-2); border-bottom: 1px solid var(--border)">
                        <input
                            type="search"
                            bind:this={nsSearchInput}
                            bind:value={searchQuery}
                            oninput={handleSearchInput}
                            placeholder="Search namespaces..."
                            aria-busy={searchLoading}
                            autocomplete="off"
                        />
                    </div>
                    {#each searchResults as ns (ns.id)}
                        <button
                            role="menuitem"
                            aria-selected={ns.name === namespace}
                            onclick={() => selectNamespace(ns)}
                        >
                            {ns.name}
                        </button>
                    {/each}
                    {#if searchResults.length === 0 && !searchLoading}
                        <div class="text-lighter align-center" style="padding: var(--space-3); font-size: var(--text-7)">
                            {searchQuery ? "No namespaces found" : "No namespaces available"}
                        </div>
                    {/if}
                    {#if searchLoading}
                        <div class="text-lighter align-center" style="padding: var(--space-3); font-size: var(--text-7)">
                            Searching...
                        </div>
                    {/if}
                </div>
            </ot-dropdown>
        {/if}
    </header>

    <nav>
        <ul>
            {#if permissions.flows.canRead}
                <li><a href="/view/{encodeURIComponent(namespace)}/flows" aria-current={isActiveLink("flows") ? "page" : undefined} title={isCollapsed ? "Flows" : ""} class:icon-only={isCollapsed}>
                    <IconGridDots size={20} aria-hidden="true" /> {#if !isCollapsed}Flows{/if}
                </a></li>
            {/if}
            {#if permissions.nodes.canRead}
                <li><a href="/view/{encodeURIComponent(namespace)}/nodes" aria-current={isActiveLink("nodes") ? "page" : undefined} title={isCollapsed ? "Nodes" : ""} class:icon-only={isCollapsed}>
                    <IconServer size={20} aria-hidden="true" /> {#if !isCollapsed}Nodes{/if}
                </a></li>
            {/if}
            {#if permissions.credentials.canRead}
                <li><a href="/view/{encodeURIComponent(namespace)}/credentials" aria-current={isActiveLink("credentials") ? "page" : undefined} title={isCollapsed ? "Credentials" : ""} class:icon-only={isCollapsed}>
                    <IconKey size={20} aria-hidden="true" /> {#if !isCollapsed}Credentials{/if}
                </a></li>
            {/if}
            {#if permissions.secrets.canRead}
                <li><a href="/view/{encodeURIComponent(namespace)}/secrets" aria-current={isActiveLink("secrets") ? "page" : undefined} title={isCollapsed ? "Secrets" : ""} class:icon-only={isCollapsed}>
                    <IconLock size={20} aria-hidden="true" /> {#if !isCollapsed}Secrets{/if}
                </a></li>
            {/if}
            {#if permissions.members.canRead}
                <li><a href="/view/{encodeURIComponent(namespace)}/members" aria-current={isActiveLink("members") ? "page" : undefined} title={isCollapsed ? "Members" : ""} class:icon-only={isCollapsed}>
                    <IconUsers size={20} aria-hidden="true" /> {#if !isCollapsed}Members{/if}
                </a></li>
            {/if}
            {#if permissions.approvals.canRead}
                <li><a href="/view/{encodeURIComponent(namespace)}/approvals" aria-current={isActiveLink("approvals") ? "page" : undefined} title={isCollapsed ? "Approvals" : ""} class:icon-only={isCollapsed}>
                    <IconCircleCheck size={20} aria-hidden="true" /> {#if !isCollapsed}Approvals{/if}
                </a></li>
            {/if}
            {#if permissions.history.canRead}
                <li><a href="/view/{encodeURIComponent(namespace)}/history" aria-current={isActiveLink("history") ? "page" : undefined} title={isCollapsed ? "History" : ""} class:icon-only={isCollapsed}>
                    <IconClock size={20} aria-hidden="true" /> {#if !isCollapsed}History{/if}
                </a></li>
            {/if}
        </ul>
    </nav>

    <footer>
        <div class="hstack items-center footer-actions" class:collapsed-footer={isCollapsed}>
            {#if $currentUser && $currentUser.role === "superuser"}
                <a href="/settings" class="button ghost icon small" aria-current={isActiveLink("settings") ? "page" : undefined} title="Settings">
                    <IconSettings size={18} aria-hidden="true" />
                </a>
            {/if}
            <ThemeToggle collapsed={true} />
            <button type="button" onclick={toggleCollapse} class="ghost icon small" aria-label={isCollapsed ? "Expand sidebar" : "Collapse sidebar"} title={isCollapsed ? "Expand sidebar" : "Collapse sidebar"}>
                {#if isCollapsed}
                    <IconChevronsRight size={18} />
                {:else}
                    <IconChevronsLeft size={18} />
                {/if}
            </button>
        </div>
        {#if $currentUser}
            {#if isCollapsed}
                <figure data-variant="avatar" class="small collapsed-avatar" aria-label={$currentUser.name} title={$currentUser.name}>
                    <abbr title={$currentUser.name}>{$currentUser.name.charAt(0).toUpperCase()}</abbr>
                </figure>
            {:else}
                <UserDropdown />
            {/if}
        {/if}
    </footer>
</aside>

<style>
    /* Override oat's fixed 14rem sidebar width to support collapse */
    aside[data-sidebar] {
        transition: width var(--transition);
        width: 14rem;
        overflow: hidden;
        background-color: var(--card);
    }
    aside[data-sidebar].collapsed {
        width: 4.5rem;
    }

    /* Override oat grid to use sidebar's dynamic width */
    :global([data-sidebar-layout]) {
        grid-template-columns: auto 1fr !important;
    }

    aside[data-sidebar] > :global(nav) {
        font-size: var(--text-6);
    }

    /* Better vertical alignment and padding for nav links */
    aside[data-sidebar] > :global(nav a) {
        align-items: center;
        gap: var(--space-3);
        padding: var(--space-2) var(--space-3);
    }

    .icon-only {
        justify-content: center;
        padding-inline: var(--space-2) !important;
    }

    .footer-actions {
        justify-content: space-between;
        margin-bottom: var(--space-4);
    }

    .collapsed-footer {
        flex-direction: column;
        flex-wrap: nowrap;
        align-items: center;
        align-content: center;
        justify-content: center;
        gap: var(--space-2);
        width: 100%;
    }

    .ns-collapsed-indicator {
        display: flex;
        align-items: center;
        justify-content: center;
    }

    .ns-initial {
        display: flex;
        align-items: center;
        justify-content: center;
        width: 2rem;
        height: 2rem;
        border-radius: var(--radius-medium);
        background: var(--faint);
        color: var(--primary);
        font-weight: var(--font-semibold);
        font-size: var(--text-7);
    }

    .collapsed-avatar {
        background: var(--primary);
        color: white;
        display: flex;
        margin: 0 auto;
    }
</style>
