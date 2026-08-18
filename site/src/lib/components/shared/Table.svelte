<script lang="ts" generics="T">
    import type { TableColumn, TableAction } from "$lib/types";
    import type { ComponentType } from "svelte";
    import IconClipboardList from "@tabler/icons-svelte/icons/clipboard-list";
    import IconChevronUp from "@tabler/icons-svelte/icons/chevron-up";
    import IconChevronDown from "@tabler/icons-svelte/icons/chevron-down";

    type SortDirection = "asc" | "desc" | null;

    type Props = {
        columns: TableColumn<T>[];
        data: T[];
        onRowClick?: (row: T) => void;
        actions?: TableAction<T>[];
        loading?: boolean;
        emptyMessage?: string;
        emptyIcon?: string;
        EmptyIconComponent?: ComponentType;
        emptyIconSize?: number;
        emptyActionLabel?: string;
        onEmptyAction?: () => void;
        title?: string;
        subtitle?: string;
    };

    let {
        columns,
        data,
        onRowClick,
        actions = [],
        loading = false,
        emptyMessage = "No data available",
        emptyIcon,
        EmptyIconComponent,
        emptyIconSize = 48,
        emptyActionLabel,
        onEmptyAction,
        title,
        subtitle,
    }: Props = $props();

    let sortKey = $state<string | null>(null);
    let sortDirection = $state<SortDirection>(null);
    const tableId = $props.id();

    const getValue = (row: T, column: TableColumn<T>) => {
        const keys = column.key.split(".");
        let value = row as any;

        for (const key of keys) {
            if (value && typeof value === "object") {
                value = value[key];
            } else {
                return undefined;
            }
        }

        return value;
    };

    const renderValue = (row: T, column: TableColumn<T>) => {
        const value = getValue(row, column);

        if (column.render) {
            return column.render(value, row);
        }

        return value ?? "";
    };

    const handleRowClick = (row: T) => {
        if (onRowClick) {
            onRowClick(row);
        }
    };

    const handleActionClick = (
        action: TableAction<T>,
        row: T,
        event: Event,
    ) => {
        event.stopPropagation();
        action.onClick(row, event);
    };

    const handleSort = (column: TableColumn<T>) => {
        if (!column.sortable) return;

        if (sortKey === column.key) {
            if (sortDirection === "asc") {
                sortDirection = "desc";
            } else if (sortDirection === "desc") {
                sortDirection = null;
                sortKey = null;
            } else {
                sortDirection = "asc";
            }
        } else {
            sortKey = column.key;
            sortDirection = "asc";
        }
    };

    const sortedData = $derived.by(() => {
        if (!sortKey || !sortDirection) return data;

        const column = columns.find((c) => c.key === sortKey);
        if (!column) return data;

        return [...data].sort((a, b) => {
            const aValue = getValue(a, column);
            const bValue = getValue(b, column);

            if (aValue == null && bValue == null) return 0;
            if (aValue == null) return sortDirection === "asc" ? 1 : -1;
            if (bValue == null) return sortDirection === "asc" ? -1 : 1;

            const aCompare =
                typeof aValue === "number"
                    ? aValue
                    : String(aValue).toLowerCase();
            const bCompare =
                typeof bValue === "number"
                    ? bValue
                    : String(bValue).toLowerCase();

            if (aCompare < bCompare) return sortDirection === "asc" ? -1 : 1;
            if (aCompare > bCompare) return sortDirection === "asc" ? 1 : -1;
            return 0;
        });
    });
</script>

<div class="card">
    {#if title || subtitle}
        <header>
            {#if title}
                <h3>{title}</h3>
            {/if}
            {#if subtitle}
                <p class="text-light text-sm">{subtitle}</p>
            {/if}
        </header>
    {/if}

    {#if loading}
        <div class="empty-state" role="status" aria-live="polite">
            <div class="hstack gap-3 justify-center">
                <div aria-busy="true"></div>
                <span class="text-sm text-light">Loading...</span>
            </div>
        </div>
    {:else if data.length === 0}
        <div class="empty-state">
            {#if EmptyIconComponent}
                <div class="text-light mb-4">
                    <EmptyIconComponent size={emptyIconSize} />
                </div>
            {:else if emptyIcon}
                {@html emptyIcon}
            {:else}
                <div class="text-light mb-4">
                    <IconClipboardList size={48} />
                </div>
            {/if}
            <h3>{emptyMessage}</h3>
            {#if emptyActionLabel && onEmptyAction}
                <button
                    onclick={onEmptyAction}
                    data-variant="secondary"
                    class="mt-4"
                    style="font-size: var(--text-7)"
                >
                    {emptyActionLabel}
                </button>
            {/if}
        </div>
    {:else}
        <div class="table">
            <table>
                <thead>
                <tr>
                    {#each columns as column}
                        <th
                            scope="col"
                            class={column.sortable ? 'sortable' : ''}
                            onclick={() => handleSort(column)}
                            aria-sort={sortKey === column.key
                                ? sortDirection === "asc"
                                    ? "ascending"
                                    : "descending"
                                : undefined}
                        >
                            <div class="hstack gap-1">
                                <span>{column.header}</span>
                                {#if column.sortable}
                                    <div class="sort-indicators" aria-hidden="true">
                                        <IconChevronUp size={14} class="sort-arrow {sortKey === column.key && sortDirection === 'asc' ? 'active' : ''}" />
                                        <IconChevronDown size={14} class="sort-arrow sort-arrow-down {sortKey === column.key && sortDirection === 'desc' ? 'active' : ''}" />
                                    </div>
                                {/if}
                            </div>
                        </th>
                    {/each}
                    {#if actions.length > 0}
                        <th scope="col">
                            <span class="sr-only">Actions</span>
                        </th>
                    {/if}
                </tr>
            </thead>
            <tbody>
                {#each sortedData as row, rowIndex}
                    <tr
                        class={onRowClick ? 'clickable' : ''}
                        onclick={() => handleRowClick(row)}
                    >
                        {#each columns as column}
                            <td>
                                {#if column.component}
                                    {@const Component = column.component}
                                    <Component
                                        {row}
                                        value={getValue(row, column)}
                                        {...(column.componentProps || {})}
                                    />
                                {:else}
                                    {@html renderValue(row, column)}
                                {/if}
                            </td>
                        {/each}
                        {#if actions.length > 0}
                            {@const visibleActions = actions.filter(a => !a.visible || a.visible(row))}
                            <td class="actions-cell" onclick={(e) => e.stopPropagation()}>
                                {#if visibleActions.length > 0}
                                    <ot-dropdown>
                                        <button
                                            popovertarget="table-actions-{tableId}-{rowIndex}"
                                            class="ghost icon small"
                                            aria-label="Actions"
                                        >
                                            <svg width="20" height="20" fill="currentColor" viewBox="0 0 20 20">
                                                <circle cx="10" cy="4" r="1.5" />
                                                <circle cx="10" cy="10" r="1.5" />
                                                <circle cx="10" cy="16" r="1.5" />
                                            </svg>
                                        </button>
                                        <div popover id="table-actions-{tableId}-{rowIndex}" role="menu">
                                            {#each visibleActions as action}
                                                {#if action.href}
                                                    <a
                                                        href={action.href(row)}
                                                        role="menuitem"
                                                        class={action.className || ''}
                                                    >
                                                        {action.label}
                                                    </a>
                                                {:else}
                                                    <button
                                                        role="menuitem"
                                                        onclick={(e) => handleActionClick(action, row, e)}
                                                        class={action.className || ''}
                                                    >
                                                        {action.label}
                                                    </button>
                                                {/if}
                                            {/each}
                                        </div>
                                    </ot-dropdown>
                                {/if}
                            </td>
                        {/if}
                    </tr>
                {/each}
            </tbody>
        </table>
        </div>
    {/if}
</div>

<style>
    .empty-state {
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        height: 16rem;
        text-align: center;
        color: var(--muted-foreground);
    }

    .empty-state h3 {
        font-size: var(--text-6);
        font-weight: var(--font-medium);
        margin: 0;
        color: var(--muted-foreground);
    }

    th.sortable {
        cursor: pointer;
        user-select: none;
    }

    .sort-indicators {
        display: flex;
        flex-direction: column;
    }

    .sort-arrow {
        width: 0.75rem;
        height: 0.75rem;
        color: var(--muted-foreground);
    }

    .sort-arrow-down {
        margin-top: -0.25rem;
    }

    .sort-arrow.active {
        color: var(--primary);
    }

    tr.clickable {
        cursor: pointer;
    }

    .actions-cell {
        width: 4rem;
        text-align: right;
    }

    .sr-only {
        position: absolute;
        width: 1px;
        height: 1px;
        padding: 0;
        margin: -1px;
        overflow: hidden;
        clip: rect(0, 0, 0, 0);
        white-space: nowrap;
        border-width: 0;
    }

</style>
