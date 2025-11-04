<script lang="ts" generics="T">
    import type { TableColumn, TableAction } from "$lib/types";

    type SortDirection = "asc" | "desc" | null;

    type Props = {
        columns: TableColumn<T>[];
        data: T[];
        onRowClick?: (row: T) => void;
        actions?: TableAction<T>[];
        loading?: boolean;
        emptyMessage?: string;
        emptyIcon?: string;
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
        title,
        subtitle,
    }: Props = $props();

    let sortKey = $state<string | null>(null);
    let sortDirection = $state<SortDirection>(null);

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
            // Cycle through: asc -> desc -> null
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

            // Handle null/undefined values
            if (aValue == null && bValue == null) return 0;
            if (aValue == null) return sortDirection === "asc" ? 1 : -1;
            if (bValue == null) return sortDirection === "asc" ? -1 : 1;

            // Convert to strings for comparison if not already numbers
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

<div class="bg-white rounded-lg border border-gray-200 overflow-hidden">
    {#if title || subtitle}
        <div class="px-6 py-4 border-b border-gray-200 bg-gray-50">
            {#if title}
                <h3 class="text-lg font-semibold text-gray-900">{title}</h3>
            {/if}
            {#if subtitle}
                <p class="text-sm text-gray-600 mt-1">{subtitle}</p>
            {/if}
        </div>
    {/if}

    {#if loading}
        <div
            class="flex items-center justify-center h-64"
            role="status"
            aria-live="polite"
        >
            <div
                class="animate-spin rounded-full h-8 w-8 border-b-2 border-primary-500"
                aria-hidden="true"
            ></div>
            <span class="ml-3 text-gray-600">Loading...</span>
        </div>
    {:else if data.length === 0}
        <div class="flex flex-col items-center justify-center h-64 text-center">
            {#if emptyIcon}
                {@html emptyIcon}
            {:else}
                <svg
                    class="w-16 h-16 text-gray-400 mb-4"
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                >
                    <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="2"
                        d="M9 5H7a2 2 0 00-2 2v10a2 2 0 002 2h8a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-3 7h3m-3 4h3m-6-4h.01M9 16h.01"
                    />
                </svg>
            {/if}
            <h3 class="text-lg font-medium text-gray-900 mb-2">
                {emptyMessage}
            </h3>
        </div>
    {:else}
        <div class="overflow-x-auto">
            <table class="min-w-full divide-y divide-gray-200">
                <thead class="bg-gray-50">
                <tr>
                    {#each columns as column}
                        <th
                            scope="col"
                            class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider {column.width
                                ? column.width
                                : ''} {column.sortable
                                ? 'cursor-pointer select-none hover:bg-gray-100'
                                : ''}"
                            onclick={() => handleSort(column)}
                            aria-sort={sortKey === column.key
                                ? sortDirection === "asc"
                                    ? "ascending"
                                    : "descending"
                                : undefined}
                        >
                            <div class="flex items-center space-x-1">
                                <span>{column.header}</span>
                                {#if column.sortable}
                                    <div
                                        class="flex flex-col"
                                        aria-hidden="true"
                                    >
                                        <svg
                                            class="w-3 h-3 {sortKey ===
                                                column.key &&
                                            sortDirection === 'asc'
                                                ? 'text-primary-500'
                                                : 'text-gray-400'}"
                                            fill="currentColor"
                                            viewBox="0 0 20 20"
                                        >
                                            <path
                                                fill-rule="evenodd"
                                                d="M14.707 12.707a1 1 0 01-1.414 0L10 9.414l-3.293 3.293a1 1 0 01-1.414-1.414l4-4a1 1 0 011.414 0l4 4a1 1 0 010 1.414z"
                                                clip-rule="evenodd"
                                            />
                                        </svg>
                                        <svg
                                            class="w-3 h-3 -mt-1 {sortKey ===
                                                column.key &&
                                            sortDirection === 'desc'
                                                ? 'text-primary-500'
                                                : 'text-gray-400'}"
                                            fill="currentColor"
                                            viewBox="0 0 20 20"
                                        >
                                            <path
                                                fill-rule="evenodd"
                                                d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z"
                                                clip-rule="evenodd"
                                            />
                                        </svg>
                                    </div>
                                {/if}
                            </div>
                        </th>
                    {/each}
                    {#if actions.length > 0}
                        <th
                            scope="col"
                            class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider w-32"
                        >
                            Actions
                        </th>
                    {/if}
                </tr>
            </thead>
            <tbody class="bg-white divide-y divide-gray-200">
                {#each sortedData as row}
                    <tr
                        class="hover:bg-gray-50 {onRowClick
                            ? 'cursor-pointer'
                            : ''}"
                        onclick={() => handleRowClick(row)}
                    >
                        {#each columns as column}
                            <td
                                class="px-6 py-4 whitespace-nowrap {column.width
                                    ? column.width
                                    : ''}"
                            >
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
                            <td
                                class="px-6 py-4 whitespace-nowrap text-sm font-medium w-32"
                            >
                                {#each actions as action}
                                    <button
                                        onclick={(e) =>
                                            handleActionClick(action, row, e)}
                                        class="{action.className ||
                                            'text-primary-500 hover:text-primary-900'} mr-3 cursor-pointer"
                                        aria-label={action.label}
                                    >
                                        {action.label}
                                    </button>
                                {/each}
                            </td>
                        {/if}
                    </tr>
                {/each}
            </tbody>
        </table>
        </div>
    {/if}
</div>
