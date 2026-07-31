<script lang="ts">
	import type { PageData } from './$types';
	import Header from '$lib/components/shared/Header.svelte';
	import PageHeader from '$lib/components/shared/PageHeader.svelte';
	import UsersTab from '$lib/components/settings/UsersTab.svelte';
	import GroupsTab from '$lib/components/settings/GroupsTab.svelte';
	import NamespacesTab from '$lib/components/settings/NamespacesTab.svelte';
	import { appInfo } from '$lib/stores/auth';

	let { data }: { data: PageData } = $props();

	let refreshTrigger = $state(false);

	function handleTabChange() {
		refreshTrigger = !refreshTrigger;
	}
</script>

<svelte:head>
	<title>Settings - {$appInfo?.branding?.app_name || "Flowctl"}</title>
</svelte:head>

<Header breadcrumbs={[{ label: "Settings" }]}>
</Header>

<div class="page-content">
	<PageHeader
		title="Settings"
		subtitle="Manage global application settings and user administration"
	/>

	<ot-tabs onot-tab-change={handleTabChange}>
		<div role="tablist">
			<button role="tab">Namespaces <span class="badge">{data.namespacesTotalCount}</span></button>
			<button role="tab" aria-selected="true">Users <span class="badge">{data.usersTotalCount}</span></button>
			<button role="tab">Groups <span class="badge">{data.groupsTotalCount}</span></button>
		</div>
		<div role="tabpanel">
			<NamespacesTab
				namespaces={data.namespaces}
				totalCount={data.namespacesTotalCount}
				pageCount={data.namespacesPageCount}
				{refreshTrigger}
			/>
		</div>
		<div role="tabpanel">
			<UsersTab
				users={data.users}
				totalCount={data.usersTotalCount}
				pageCount={data.usersPageCount}
				groups={data.groups}
				{refreshTrigger}
			/>
		</div>
		<div role="tabpanel">
			<GroupsTab
				groups={data.groups}
				totalCount={data.groupsTotalCount}
				pageCount={data.groupsPageCount}
				{refreshTrigger}
			/>
		</div>
	</ot-tabs>
</div>
