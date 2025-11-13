<script lang="ts">
	import type { PageData } from './$types';
	import Header from '$lib/components/shared/Header.svelte';
	import PageHeader from '$lib/components/shared/PageHeader.svelte';
	import Tabs from '$lib/components/shared/Tabs.svelte';
	import UsersTab from '$lib/components/settings/UsersTab.svelte';
	import GroupsTab from '$lib/components/settings/GroupsTab.svelte';
	import NamespacesTab from '$lib/components/settings/NamespacesTab.svelte';

	let { data }: { data: PageData } = $props();

	// Tab state
	let activeTab = $state('users');
	let refreshTrigger = $state(false);

	// Tab configuration
	const tabs = [
		{
			id: 'namespaces',
			label: 'Namespaces',
			badge: data.namespacesTotalCount
		},
		{
			id: 'users',
			label: 'Users',
			badge: data.usersTotalCount
		},
		{
			id: 'groups',
			label: 'Groups',
			badge: data.groupsTotalCount
		}
	];

	function handleTabChange(event: CustomEvent<{ tabId: string }>) {
		activeTab = event.detail.tabId;
		// Toggle refresh trigger when tab changes
		refreshTrigger = !refreshTrigger;
	}
</script>

<svelte:head>
	<title>Settings - Flowctl</title>
</svelte:head>

<Header breadcrumbs={[{ label: "Settings" }]}>
	{#snippet children()}
		<!-- Search will be handled within each tab -->
	{/snippet}
</Header>

<div class="p-12">
	<!-- Page Header -->
	<PageHeader 
		title="Settings"
		subtitle="Manage global application settings and user administration"
	/>

	<!-- Tab Navigation -->
	<div class="mb-6">
		<Tabs
			{tabs}
			bind:activeTab
			on:change={handleTabChange}
		/>
	</div>

	<!-- Tab Content -->
	{#if activeTab === 'users'}
		<UsersTab
			users={data.users}
			totalCount={data.usersTotalCount}
			pageCount={data.usersPageCount}
			groups={data.groups}
			{refreshTrigger}
		/>
	{:else if activeTab === 'groups'}
		<GroupsTab
			groups={data.groups}
			totalCount={data.groupsTotalCount}
			pageCount={data.groupsPageCount}
			{refreshTrigger}
		/>
	{:else if activeTab === 'namespaces'}
		<NamespacesTab
			namespaces={data.namespaces}
			totalCount={data.namespacesTotalCount}
			pageCount={data.namespacesPageCount}
			{refreshTrigger}
		/>
	{/if}
</div>