<script lang="ts">
	import { browser } from '$app/environment';
	import { page } from '$app/state';
	import type { PageData } from './$types';
	import PageHeader from '$lib/components/shared/PageHeader.svelte';
	import Table from '$lib/components/shared/Table.svelte';
	import MemberCell from '$lib/components/members/MemberCell.svelte';
	import MemberTypeBadge from '$lib/components/members/MemberTypeBadge.svelte';
	import MemberRoleBadge from '$lib/components/members/MemberRoleBadge.svelte';
	import AddMemberModal from '$lib/components/members/AddMemberModal.svelte';
	import { apiClient } from '$lib/apiClient';
	import type { NamespaceMemberResp, NamespaceMemberReq } from '$lib/types';
	import Header from '$lib/components/shared/Header.svelte';

	let { data }: { data: PageData } = $props();

	// State
	let members = $state(data.members);
	let loading = $state(false);
	let showAddModal = $state(false);
	let showRemoveModal = $state(false);
	let removeMemberId = $state<string | null>(null);
	let removeMemberName = $state('');

	// Table configuration
	let tableColumns = [
		{
			key: 'subject_name',
			header: 'Member',
			component: MemberCell
		},
		{
			key: 'subject_type',
			header: 'Type',
			component: MemberTypeBadge
		},
		{
			key: 'role',
			header: 'Role',
			component: MemberRoleBadge
		},
		{
			key: 'created_at',
			header: 'Added',
			render: (_value: any, member: NamespaceMemberResp) => formatDate(member.created_at)
		}
	];

	let tableActions = [
		{
			label: 'Remove',
			onClick: (member: NamespaceMemberResp) => handleRemove(member.id, member.subject_name),
			className: 'text-red-600 hover:text-red-800'
		}
	];

	// Functions
	async function fetchMembers() {
		if (!browser) return;
		
		loading = true;
		try {
			const response = await apiClient.namespaces.members.list(data.namespace);
			members = response.members || [];
		} catch (error) {
			console.error('Failed to fetch members:', error);
			notifyError('Failed to fetch members');
		} finally {
			loading = false;
		}
	}

	async function handleMemberSave(memberData: NamespaceMemberReq) {
		try {
			await apiClient.namespaces.members.add(data.namespace, memberData);
			showAddModal = false;
			await fetchMembers();
			notifySuccess('Member added successfully');
		} catch (error) {
			console.error('Failed to add member:', error);
			notifyError('Failed to add member');
			throw error; // Re-throw so modal can handle it
		}
	}

	function handleAdd() {
		showAddModal = true;
	}

	function handleRemove(memberId: string, memberName: string) {
		removeMemberId = memberId;
		removeMemberName = memberName;
		showRemoveModal = true;
	}

	async function confirmRemove() {
		if (!removeMemberId) return;

		try {
			await apiClient.namespaces.members.remove(data.namespace, removeMemberId);
			showRemoveModal = false;
			removeMemberId = null;
			removeMemberName = '';
			await fetchMembers();
			notifySuccess('Member removed successfully');
		} catch (error) {
			console.error('Failed to remove member:', error);
			notifyError('Failed to remove member');
		}
	}

	function closeAddModal() {
		showAddModal = false;
	}

	function closeRemoveModal() {
		showRemoveModal = false;
		removeMemberId = null;
		removeMemberName = '';
	}

	function formatDate(dateString: string): string {
		if (!dateString) return 'Unknown';
		const date = new Date(dateString);
		const now = new Date();
		const diffMs = now.getTime() - date.getTime();
		const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));

		if (diffDays === 0) return 'Today';
		if (diffDays === 1) return 'Yesterday';
		if (diffDays < 7) return `${diffDays} days ago`;
		return date.toLocaleDateString();
	}

	function notifySuccess(message: string) {
		window.dispatchEvent(
			new CustomEvent("notify", {
				detail: { message, type: "success" },
			})
		);
	}

	function notifyError(message: string) {
		window.dispatchEvent(
			new CustomEvent("notify", {
				detail: { message, type: "error" },
			})
		);
	}
</script>

<svelte:head>
  <title>Members - {page.params.namespace} - Flowctl</title>
</svelte:head>

<Header breadcrumbs={[`${page.params.namespace}`, "Members"]}>
  {#snippet children()}
    <!-- Empty slot for now -->
  {/snippet}
</Header>

<div class="p-12">
	<!-- Page Header -->
	<PageHeader 
		title="Members"
		subtitle="Manage user and group access to this namespace"
		actions={[
			{
				label: 'Add Member',
				onClick: handleAdd,
				variant: 'primary',
				icon: '<svg class="w-4 h-4 inline mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6v6m0 0v6m0-6h6m-6 0H6"></path></svg>'
			}
		]}
	/>

	<!-- Members Table -->
	<div class="pt-6">
		<Table
			data={members}
			columns={tableColumns}
			actions={tableActions}
			{loading}
			emptyMessage="No members found. Get started by adding users or groups to this namespace."
		/>
	</div>
</div>

<!-- Add Member Modal -->
<AddMemberModal
	bind:show={showAddModal}
	onSave={handleMemberSave}
	onClose={closeAddModal}
/>

<!-- Remove Member Confirmation Modal -->
{#if showRemoveModal}
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-gray-900/60 p-4" on:click={closeRemoveModal}>
		<div class="bg-white rounded-lg shadow-lg w-full max-w-md" on:click|stopPropagation>
			<div class="p-6">
				<div class="flex items-center mb-4">
					<div class="w-12 h-12 bg-red-100 rounded-lg flex items-center justify-center mr-4">
						<svg class="w-6 h-6 text-red-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.964-.833-2.732 0L3.732 16.5c-.77.833.192 2.5 1.732 2.5z"/>
						</svg>
					</div>
					<div>
						<h3 class="text-lg font-semibold text-gray-900">Remove Member</h3>
						<p class="text-sm text-gray-600">This action cannot be undone.</p>
					</div>
				</div>

				<p class="text-gray-700 mb-6">
					Are you sure you want to remove "<span class="font-medium">{removeMemberName}</span>" from this namespace?
				</p>

				<div class="flex justify-end gap-2">
					<button 
						on:click={closeRemoveModal}
						class="inline-flex items-center px-5 py-2.5 text-sm font-medium text-gray-700 bg-gray-100 rounded-lg hover:bg-gray-200"
					>
						Cancel
					</button>
					<button 
						on:click={confirmRemove}
						class="inline-flex items-center px-5 py-2.5 text-sm font-medium text-white bg-red-600 rounded-lg hover:bg-red-700"
					>
						Remove Member
					</button>
				</div>
			</div>
		</div>
	</div>
{/if}