<script lang="ts">
	import type { FlowInput } from '$lib/types';
	import { onMount } from 'svelte';

	let {
		inputs = [],
		values = $bindable({}),
		errors = {},
		useFormData = false
	}: {
		inputs?: FlowInput[];
		values?: Record<string, any>;
		errors?: Record<string, string>;
		useFormData?: boolean;
	} = $props();

	// Track loading state and merged options for each input
	let remoteOptionsLoading: Record<string, boolean> = $state({});
	let mergedOptions: Record<string, string[]> = $state({});

	// Interpolate variables in URL
	function interpolateVariables(url: string, vars: Record<string, any>): string {
		let result = url;
		const pattern = /\{\{(\w+)\}\}/g;
		result = result.replace(pattern, (match, varName) => {
			const value = vars[varName];
			return value !== undefined ? String(value) : match;
		});
		return result;
	}

	// Fetch options from remote URL
	async function fetchRemoteOptions(input: FlowInput) {
		if (!input.options_url) {
			return;
		}

		remoteOptionsLoading[input.name] = true;

		try {
			// Interpolate variables in the URL
			const url = interpolateVariables(input.options_url, values);

			const response = await fetch(url);
			if (!response.ok) {
				console.error(`Failed to fetch options from ${url}: ${response.statusText}`);
				return;
			}

			const data = await response.json();
			const remote = Array.isArray(data) ? data : [];

			// Extract option names from the response
			const remoteOptionNames = remote
				.filter(opt => opt && opt.name)
				.map(opt => opt.name);

			// Merge with static options
			const merged = [...remoteOptionNames];
			for (const staticOpt of input.options || []) {
				if (!merged.includes(staticOpt)) {
					merged.push(staticOpt);
				}
			}

			mergedOptions[input.name] = merged;
		} catch (error) {
			console.error(`Error fetching remote options for ${input.name}:`, error);
		} finally {
			remoteOptionsLoading[input.name] = false;
		}
	}

	// Load remote options when component mounts or when inputs change
	onMount(async () => {
		for (const input of inputs) {
			if (input.type === 'select' && input.options_url) {
				await fetchRemoteOptions(input);
			}
		}
	});

	// Update mergedOptions when inputs change
	$effect.pre(() => {
		inputs;
		for (const input of inputs) {
			if (input.type === 'select' && input.options_url) {
				if (!mergedOptions[input.name]) {
					fetchRemoteOptions(input);
				}
			}
		}
	});
</script>

{#if inputs && inputs.length > 0}
	<div class="space-y-4">
	{#each inputs as input (input.name)}
		<div>
			<label for={input.name} class="block text-sm font-medium text-foreground mb-2">
				{input.label || input.name}
				{#if input.required}
					<span class="text-red-500">*</span>
				{/if}
			</label>

			{#if errors[input.name]}
				<p class="text-sm text-danger-600 mb-2">{errors[input.name]}</p>
			{/if}

			{#if input.type === 'string' || input.type === 'number'}
				{#if useFormData}
					<input
						type={input.type === 'string' ? 'text' : 'number'}
						id={input.name}
						name={input.name}
						value={values[input.name] ?? input.default ?? ''}
						placeholder={input.description || ''}
						required={input.required}
						class="w-full px-3 py-2 text-foreground bg-card border border-input rounded-md focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent"
					/>
				{:else}
					<input
						type={input.type === 'string' ? 'text' : 'number'}
						bind:value={values[input.name]}
						placeholder={input.description || ''}
						required={input.required}
						class="w-full px-3 py-2 text-foreground bg-card border border-input rounded-md focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent"
					/>
				{/if}
			{:else if input.type === 'checkbox'}
				<div class="flex items-center">
					{#if useFormData}
						<input
							type="checkbox"
							id={input.name}
							name={input.name}
							value="true"
							checked={values[input.name] ?? input.default === 'true'}
							class="h-4 w-4 text-primary-600 focus:ring-primary-500 border-input rounded"
						/>
					{:else}
						<input
							type="checkbox"
							bind:checked={values[input.name]}
							class="h-4 w-4 text-primary-600 focus:ring-primary-500 border-input rounded"
						/>
					{/if}
				</div>
			{:else if input.type === 'select' && (input.options || input.options_url)}
				{@const isLoading = remoteOptionsLoading[input.name]}
				{@const options = mergedOptions[input.name] || input.options || []}
				{#if isLoading}
					<div class="flex items-center gap-2 px-3 py-2 text-foreground bg-card border border-input rounded-md text-sm">
						<svg class="animate-spin w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
							<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
						</svg>
						Loading options...
					</div>
					</div>
				{:else if useFormData}
					<select
						id={input.name}
						name={input.name}
						required={input.required}
						value={values[input.name] ?? input.default ?? ''}
						class="w-full px-3 py-2 text-foreground bg-card border border-input rounded-md focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent"
					>
						<option value="">Select an option</option>
						{#each options as option}
							<option value={option} selected={option === (values[input.name] ?? input.default)}
								>{option}</option
							>
						{/each}
					</select>
				{:else}
					<select
						bind:value={values[input.name]}
						required={input.required}
						class="w-full px-3 py-2 text-foreground bg-card border border-input rounded-md focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent"
					>
						<option value="">Select an option</option>
						{#each options as option}
							<option value={option}>{option}</option>
						{/each}
					</select>
				{/if}
			{:else if input.type === 'file'}
				<div class="flex flex-col">
					<input
						type="file"
						id={input.name}
						name={input.name}
						required={input.required}
						class="w-full px-3 py-2 text-foreground bg-card border border-input rounded-md focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent file:mr-4 file:py-2 file:px-4 file:rounded-md file:border-0 file:text-sm file:font-medium file:bg-primary-50 file:text-primary-700 hover:file:bg-primary-100"
					/>
					{#if input.max_file_size}
						<p class="text-xs text-muted-foreground mt-1">
							Max size: {Math.round(input.max_file_size / (1024 * 1024))}MB
						</p>
					{/if}
				</div>
			{:else if input.type === 'datetime'}
				<div class="flex items-center">
					{#if useFormData}
						<input
							type="datetime-local"
							id={input.name}
							name={input.name}
							value={values[input.name] ?? input.default ?? ''}
							required={input.required}
							class="w-full px-3 py-2 text-foreground bg-card border border-input rounded-md focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent"
						/>
					{:else}
						<input
							type="datetime-local"
							bind:value={values[input.name]}
							required={input.required}
							class="w-full px-3 py-2 text-foreground bg-card border border-input rounded-md focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent"
						/>
					{/if}
				</div>
			{:else if input.type === 'password'}
				<div class="flex items-center">
					{#if useFormData}
						<input
							type="password"
							id={input.name}
							name={input.name}
							value={values[input.name] ?? input.default ?? ''}
							required={input.required}
							class="w-full px-3 py-2 text-foreground bg-card border border-input rounded-md focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent"
						/>
					{:else}
						<input
							type="password"
							bind:value={values[input.name]}
							required={input.required}
							class="w-full px-3 py-2 text-foreground bg-card border border-input rounded-md focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent"
						/>
					{/if}
				</div>
			{:else}
				<!-- Fallback for other input types -->
				{#if useFormData}
					<input
						type="text"
						id={input.name}
						name={input.name}
						value={values[input.name] ?? input.default ?? ''}
						placeholder={input.description || ''}
						required={input.required}
						class="w-full px-3 py-2 text-foreground bg-card border border-input rounded-md focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent"
					/>
				{:else}
					<input
						type="text"
						bind:value={values[input.name]}
						placeholder={input.description || ''}
						required={input.required}
						class="w-full px-3 py-2 text-foreground bg-card border border-input rounded-md focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent"
					/>
				{/if}
			{/if}

			{#if input.description}
				<p class="text-sm text-muted-foreground mt-1">{input.description}</p>
			{/if}
		</div>
	{/each}
	</div>
{/if}
