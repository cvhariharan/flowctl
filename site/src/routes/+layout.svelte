<script lang="ts">
	import '../app.css';
	import favicon from '$lib/assets/favicon.svg';
	import { currentUser, isAuthenticated, isLoading } from '$lib/stores/auth';
	import NotificationPopup from '$lib/components/shared/NotificationPopup.svelte';
	
	let { children, data } = $props();

	// When data from the load function changes, update the reactive stores
	$effect(() => {
		currentUser.set(data.user);
		isAuthenticated.set(!!data.user);
		isLoading.set(false);
	});
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
	<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@tabler/icons-webfont@latest/tabler-icons.min.css">
</svelte:head>

{@render children?.()}

<NotificationPopup />
