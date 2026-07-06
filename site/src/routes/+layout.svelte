<script lang="ts">
	import '../app.css';
	import '@knadh/oat/oat.min.js';
	import favicon from '$lib/assets/favicon.svg';
	import { currentUser, isAuthenticated, isLoading, appInfo } from '$lib/stores/auth';
	import { resolvedTheme, applyTheme } from '$lib/stores/theme';
	import { beforeNavigate } from '$app/navigation';
	import GlobalLoadingIndicator from '$lib/components/shared/GlobalLoadingIndicator.svelte';

	// Clear persistent toasts (e.g. error toasts with duration: 0) on navigation
	beforeNavigate(() => {
		if (typeof (window as any).ot?.toast?.clear === 'function') {
			(window as any).ot.toast.clear();
		}
	});

	let { children, data } = $props();

	$effect(() => {
		applyTheme($resolvedTheme);
	});

	// Update stores when userPromise resolves
	$effect(() => {
		let cancelled = false;

		data.userPromise
			.then((user) => {
				if (!cancelled) {
					currentUser.set(user);
					isAuthenticated.set(!!user);
					isLoading.set(false);
				}
			})
			.catch((error) => {
				if (!cancelled) {
					console.error('[Auth] Failed to load user profile:', error);
					currentUser.set(null);
					isAuthenticated.set(false);
					isLoading.set(false);
				}
			});

		data.infoPromise.then((info) => {
			if (!cancelled) {
				appInfo.set(info);
			}
		});

		return () => {
			cancelled = true;
		};
	});
</script>

<svelte:head>
	<link rel="icon" href={$appInfo?.branding?.favicon_url || favicon} />
	<link rel="preconnect" href="https://fonts.googleapis.com">
	<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
	<link href="https://fonts.googleapis.com/css2?family=Bitter:ital,wght@0,100..900;1,100..900&family=Inter:ital,opsz,wght@0,14..32,100..900;1,14..32,100..900&display=swap" rel="stylesheet">
	{#if $appInfo?.branding?.primary_color || $appInfo?.branding?.sidebar_color}
		{@const primary = $appInfo.branding.primary_color}
		{@const sidebar = $appInfo.branding.sidebar_color}
		{@html `<style>html:root,html:root[data-theme="dark"],html:root[data-theme="light"],html body[data-theme="dark"],html body[data-theme="light"]{${primary ? `--primary:${primary};` : ''}${sidebar ? '--card:' + sidebar + ';' : ''}}</style>`}
	{/if}
	{#if $appInfo?.branding?.app_name}
		<title>{$appInfo.branding.app_name}</title>
	{/if}
</svelte:head>

{@render children?.()}

<GlobalLoadingIndicator />
