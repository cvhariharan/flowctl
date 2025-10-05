<script lang="ts">
	import type { ComponentType } from 'svelte';

	interface Props {
		title: string;
		value: number | string;
		icon?: string;
		IconComponent?: ComponentType;
		iconSize?: number;
		color?: 'blue' | 'green' | 'purple' | 'red' | 'yellow' | 'gray';
	}

	let { title, value, icon, IconComponent, iconSize = 24, color = 'blue' }: Props = $props();

	const colorClasses = {
		blue: 'bg-info-100 text-info-600',
		green: 'bg-success-100 text-success-600',
		purple: 'bg-primary-100 text-primary-600',
		red: 'bg-danger-100 text-danger-600',
		yellow: 'bg-warning-100 text-warning-600',
		gray: 'bg-gray-100 text-gray-600'
	};
</script>

<div class="bg-white rounded-lg border border-gray-200 p-6">
	<div class="flex items-center justify-between">
		<div>
			<p class="text-sm font-medium text-gray-600">{title}</p>
			<p class="text-2xl font-bold text-gray-900">{value}</p>
		</div>
		<div class="w-12 h-12 {colorClasses[color]} rounded-lg flex items-center justify-center">
			{#if IconComponent}
				<IconComponent size={iconSize} />
			{:else if icon}
				{@html icon}
			{/if}
		</div>
	</div>
</div>