<script lang="ts">
	import { browser } from '$app/environment';
	import { page } from '$app/state';
	import type { PageData } from './$types';
	import PageHeader from '$lib/components/shared/PageHeader.svelte';
	import Table from '$lib/components/shared/Table.svelte';
	import MemberCell from '$lib/components/members/MemberCell.svelte';
	import MemberTypeBadge from '$lib/components/members/MemberTypeBadge.svelte';
	import MemberRoleBadge from '$lib/components/members/MemberRoleBadge.svelte';
	import MemberModal from '$lib/components/members/MemberModal.svelte';
	import DeleteModal from '$lib/components/shared/DeleteModal.svelte';
	import { apiClient } from '$lib/apiClient';
	import type { NamespaceMemberResp, NamespaceMemberReq } from '$lib/types';
	import Header from '$lib/components/shared/Header.svelte';
	import { handleInlineError, showSuccess } from '$lib/utils/errorHandling';
	import type { TableAction } from '$lib/types';
	import { formatDateTime } from '$lib/utils';

	let { data }: { data: PageData } = $props();

	// State
	let members = $state(data.members);
	let loading = $state(false);
	let showMemberModal = $state(false);
	let showDeleteModal = $state(false);
	let isEditMode = $state(false);
	let selectedMember = $state<NamespaceMemberResp | null>(null);
	let deleteMemberId = $state<string | null>(null);
	let deleteMemberName = $state('');
	let permissions = $state(data.permissions);

	// Table configuration
	let tableColumns = [
		{
			key: 'subject_name',
			header: 'Member',
			sortable: true,
			component: MemberCell
		},
		{
			key: 'subject_type',
			header: 'Type',
			sortable: true,
			component: MemberTypeBadge
		},
		{
			key: 'role',
			header: 'Role',
			sortable: true,
			component: MemberRoleBadge
		},
		{
			key: 'created_at',
			header: 'Added',
			sortable: true,
			render: (_value: any, member: NamespaceMemberResp) => formatDateTime(member.created_at)
		}
	];

	const tableActions = $derived((): TableAction<NamespaceMemberResp>[] => {
		const actionsList: TableAction<NamespaceMemberResp>[] = [];

		if (permissions.canUpdate) {
			actionsList.push({
				label: 'Edit',
				onClick: (member: NamespaceMemberResp) => handleEdit(member),
				className: 'text-primary-600 hover:text-primary-800'
			});
		}

		if (permissions.canDelete) {
			actionsList.push({
				label: 'Remove',
				onClick: (member: NamespaceMemberResp) => handleDelete(member.id, member.subject_name),
				className: 'text-danger-600 hover:text-danger-800'
			});
		}

		return actionsList;
	});

	// Functions
	async function fetchMembers() {
		if (!browser) return;
		
		loading = true;
		try {
			const response = await apiClient.namespaces.members.list(data.namespace);
			members = response.members || [];
		} catch (error) {
			handleInlineError(error, 'Unable to Load Members List');
		} finally {
			loading = false;
		}
	}

	async function handleMemberSave(memberData: NamespaceMemberReq) {
		try {
			if (isEditMode && selectedMember) {
				// Update existing member - only role can be updated
				await apiClient.namespaces.members.update(data.namespace, selectedMember.id, { role: memberData.role });
				showSuccess('Member Updated', 'Member updated successfully');
			} else {
				// Add new member
				await apiClient.namespaces.members.add(data.namespace, memberData);
				showSuccess('Member Added', 'Member added successfully');
			}
			closeMemberModal();
			await fetchMembers();
		} catch (error) {
			handleInlineError(error, isEditMode ? 'Unable to Update Member Role' : 'Unable to Add Member');
		}
	}

	function handleAdd() {
		isEditMode = false;
		selectedMember = null;
		showMemberModal = true;
	}

	function handleEdit(member: NamespaceMemberResp) {
		isEditMode = true;
		selectedMember = member;
		showMemberModal = true;
	}

	function handleDelete(memberId: string, memberName: string) {
		deleteMemberId = memberId;
		deleteMemberName = memberName;
		showDeleteModal = true;
	}

	async function confirmDelete() {
		if (!deleteMemberId) return;

		try {
			await apiClient.namespaces.members.remove(data.namespace, deleteMemberId);
			closeDeleteModal(); // Close modal after successful deletion
			await fetchMembers();
			showSuccess('Member Removed', 'Member removed successfully');
		} catch (error) {
			handleInlineError(error, 'Unable to Remove Member');
		}
	}

	function closeMemberModal() {
		showMemberModal = false;
		isEditMode = false;
		selectedMember = null;
	}

	function closeDeleteModal() {
		showDeleteModal = false;
		deleteMemberId = null;
		deleteMemberName = '';
	}


</script>

<svelte:head>
  <title>Members - {page.params.namespace} - Flowctl</title>
</svelte:head>

<Header breadcrumbs={[
  { label: page.params.namespace!, url: `/view/${page.params.namespace}/flows` },
  { label: "Members" }
]}>
  {#snippet children()}
    <!-- Empty slot for now -->
  {/snippet}
</Header>

<div class="p-12">
	<!-- Page Header -->
	<PageHeader 
		title="Members"
		subtitle="Manage user and group access to this namespace"
		actions={permissions.canCreate ? [
			{
				label: 'Add Member',
				onClick: handleAdd,
				variant: 'primary',
				icon: '<svg class="w-4 h-4 inline mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6v6m0 0v6m0-6h6m-6 0H6"></path></svg>'
			}
		] : []}
	/>

	<!-- Members Table -->
	<div class="pt-6">
		<Table
			data={members}
			columns={tableColumns}
			actions={tableActions()}
			{loading}
			emptyMessage="No members found. Get started by adding users or groups to this namespace."
		/>
	</div>
</div>

<!-- Member Modal (Add/Edit) -->
{#if showMemberModal}
	<MemberModal
		{isEditMode}
		memberData={selectedMember}
		onSave={handleMemberSave}
		onClose={closeMemberModal}
	/>
{/if}

<!-- Delete Modal -->
{#if showDeleteModal}
	<DeleteModal
		title="Remove Member"
		itemName={deleteMemberName}
		onConfirm={confirmDelete}
		onClose={closeDeleteModal}
	/>
{/if}