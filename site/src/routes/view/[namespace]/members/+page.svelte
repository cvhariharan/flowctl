<script lang="ts">
	import { browser } from '$app/environment';
	import { page } from '$app/state';
	import type { PageData } from './$types';
	import PageHeader from '$lib/components/shared/PageHeader.svelte';
	import Table from '$lib/components/shared/Table.svelte';
	import NameLinkCell from '$lib/components/shared/cells/NameLinkCell.svelte';
	import MutedTextCell from '$lib/components/shared/cells/MutedTextCell.svelte';
	import MemberTypeBadge from '$lib/components/members/MemberTypeBadge.svelte';
	import MemberRoleBadge from '$lib/components/members/MemberRoleBadge.svelte';
	import MemberModal from '$lib/components/members/MemberModal.svelte';
	import DeleteModal from '$lib/components/shared/DeleteModal.svelte';
	import { apiClient } from '$lib/apiClient';
	import type { NamespaceMemberResp, NamespaceMemberReq, NamespaceMembersResponse } from '$lib/types';
	import Header from '$lib/components/shared/Header.svelte';
	import { handleInlineError, showSuccess } from '$lib/utils/errorHandling';
	import type { TableAction } from '$lib/types';
	import { formatDateTime } from '$lib/utils';
	import IconUsers from '@tabler/icons-svelte/icons/users';
	import IconUser from '@tabler/icons-svelte/icons/user';
	import IconPlus from '@tabler/icons-svelte/icons/plus';

	let { data }: { data: PageData } = $props();
	const namespace = $derived(page.params.namespace!);
	const encodedNamespace = $derived(encodeURIComponent(namespace));

	// State
	let members = $state<NamespaceMemberResp[]>([]);
	let loading = $state(true);

	// Handle the async data from load function
	$effect(() => {
		let cancelled = false;

		data.membersPromise
			.then((result: NamespaceMembersResponse) => {
				if (!cancelled) {
					members = result.members || [];
					loading = false;
				}
			})
			.catch((err: Error) => {
				if (!cancelled) {
					members = [];
					handleInlineError(err, "Unable to Load Members");
					loading = false;
				}
			});

		return () => {
			cancelled = true;
		};
	});
	let showMemberModal = $state(false);
	let showDeleteModal = $state(false);
	let isEditMode = $state(false);
	let selectedMember = $state<NamespaceMemberResp | null>(null);
	let deleteMemberId = $state<string | null>(null);
	let deleteMemberName = $state('');
	let permissions = $state(data.permissions);

	// Table configuration
	let tableColumns = $derived([
		{
			key: 'subject_name',
			header: 'Member',
			sortable: true,
			component: NameLinkCell,
			componentProps: {
				getIcon: (row: NamespaceMemberResp) => row.subject_type === 'user' ? IconUser : IconUsers,
				subtitleKey: 'subject_id',
				...(permissions.canUpdate ? { onClick: handleEdit } : {})
			}
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
			component: MutedTextCell,
			componentProps: { format: (v: any) => formatDateTime(v) }
		}
	]);

	const tableActions = $derived((): TableAction<NamespaceMemberResp>[] => {
		const actionsList: TableAction<NamespaceMemberResp>[] = [];

		if (permissions.canUpdate) {
			actionsList.push({
				label: 'Edit',
				onClick: (member: NamespaceMemberResp) => handleEdit(member),
				className: 'text-link border-link hover:bg-link-hover'
			});
		}

		if (permissions.canDelete) {
			actionsList.push({
				label: 'Delete',
				onClick: (member: NamespaceMemberResp) => handleDelete(member.id, member.subject_name),
				className: 'text-danger-600 border-danger-600 hover:bg-danger-100'
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
  { label: namespace, url: `/view/${encodedNamespace}/flows` },
  { label: "Members" }
]}>

</Header>

<div class="page-content">
	<!-- Page Header -->
	<PageHeader
		title="Members"
		subtitle="Manage user and group access to this namespace"
		actions={permissions.canCreate ? [
			{
				label: 'Add',
				onClick: handleAdd,
				variant: 'primary',
				IconComponent: IconPlus,
				iconSize: 16
			}
		] : []}
	/>

	<!-- Members Table -->
	<div class="mt-4">
		<Table
			data={members}
			columns={tableColumns}
			actions={tableActions()}
			{loading}
			emptyMessage="No members found. Get started by adding users or groups to this namespace."
			EmptyIconComponent={IconUsers}
			emptyIconSize={64}
			emptyActionLabel={permissions.canCreate ? "Add your first member" : undefined}
			onEmptyAction={permissions.canCreate ? handleAdd : undefined}
		/>
	</div>
</div>

<!-- Member Modal (Add/Edit) -->
{#if showMemberModal}
	<MemberModal
		{isEditMode}
		memberData={selectedMember}
		namespace={data.namespace}
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
