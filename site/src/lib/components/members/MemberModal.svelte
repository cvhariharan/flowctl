<script lang="ts">
  import { handleInlineError } from '$lib/utils/errorHandling';
  import { autofocus } from '$lib/utils/autofocus';
  import OatSelect from '$lib/components/shared/OatSelect.svelte';
  import UserGroupSelector from '$lib/components/shared/UserGroupSelector.svelte';
  import FlowGroupSelector from '$lib/components/flow-create/FlowGroupSelector.svelte';
  import type { NamespaceMemberReq, NamespaceMemberResp, FlowGroupResp, User, Group } from '$lib/types';
  import { apiClient } from '$lib/apiClient';
  import IconUsers from '@tabler/icons-svelte/icons/users';
  import IconUser from '@tabler/icons-svelte/icons/user';
  import IconX from '@tabler/icons-svelte/icons/x';
  import IconFolder from '@tabler/icons-svelte/icons/folder';

  interface Props {
    isEditMode?: boolean;
    memberData?: NamespaceMemberResp | null;
    namespace: string;
    onSave: (memberData: NamespaceMemberReq) => void;
    onClose: () => void;
  }

  let {
    isEditMode = false,
    memberData = null,
    namespace,
    onSave,
    onClose
  }: Props = $props();

  let memberForm = $state<NamespaceMemberReq>({
    subject_type: 'user',
    subject_id: '',
    role: 'user'
  });

  let selectedSubject = $state<User | Group | null>(null);
  let loading = $state(false);

  let memberPrefixes = $state<FlowGroupResp[]>([]);
  let prefixLoading = $state(false);
  let selectedPrefix = $state('');
  let dialogEl: HTMLDialogElement;

  $effect(() => {
    if (isEditMode && memberData) {
      memberForm.subject_type = memberData.subject_type as 'user' | 'group';
      memberForm.subject_id = memberData.subject_id;
      memberForm.role = memberData.role as 'user' | 'operator' | 'reviewer' | 'admin';

      selectedSubject = {
        id: memberData.subject_id,
        name: memberData.subject_name,
        username: memberData.subject_name
      } as User | Group;

      loadMemberPrefixes();
    } else {
      resetForm();
    }
  });

  $effect(() => {
    if (selectedPrefix) {
      const alreadyAssigned = memberPrefixes.some(p => p.prefix === selectedPrefix);
      if (!alreadyAssigned) {
        addPrefix(selectedPrefix);
      }
      selectedPrefix = '';
    }
  });

  $effect(() => {
    memberForm.subject_id = selectedSubject?.id || '';
  });

  $effect(() => {
    if (dialogEl) {
      dialogEl.showModal();
    }
  });

  async function loadMemberPrefixes() {
    if (!isEditMode || !memberData) return;
    try {
      const result = await apiClient.namespaces.members.groups.list(namespace, memberData.id);
      memberPrefixes = result.groups || [];
    } catch {
      memberPrefixes = [];
    }
  }

  async function addPrefix(prefix: string) {
    if (!memberData || !prefix.trim()) return;
    prefixLoading = true;
    try {
      await apiClient.namespaces.members.groups.add(namespace, memberData.id, { prefix: prefix.trim() });
      await loadMemberPrefixes();
    } catch (err) {
      handleInlineError(err, 'Unable to Grant Group Access');
    } finally {
      prefixLoading = false;
    }
  }

  async function removePrefix(prefix: string) {
    if (!memberData) return;
    prefixLoading = true;
    try {
      await apiClient.namespaces.members.groups.remove(namespace, memberData.id, prefix);
      await loadMemberPrefixes();
    } catch (err) {
      handleInlineError(err, 'Unable to Revoke Group Access');
    } finally {
      prefixLoading = false;
    }
  }

  function onSubjectTypeChange() {
    selectedSubject = null;
    memberForm.subject_id = '';
  }

  async function handleSubmit() {
    try {
      loading = true;

      if (!selectedSubject) {
        handleInlineError(new Error('Please select a member'), 'Validation Error');
        return;
      }

      if (!memberForm.role) {
        handleInlineError(new Error('Please select a role'), 'Validation Error');
        return;
      }

      onSave(memberForm);
    } catch (err) {
      handleInlineError(err, isEditMode ? 'Unable to Update Member Role' : 'Unable to Add Member to Namespace');
    } finally {
      loading = false;
    }
  }

  function resetForm() {
    memberForm = {
      subject_type: 'user',
      subject_id: '',
      role: 'user'
    };
    selectedSubject = null;
    memberPrefixes = [];
    selectedPrefix = '';
  }
</script>

<dialog bind:this={dialogEl} onclose={onClose}>
  <form onsubmit={(e) => { e.preventDefault(); handleSubmit(); }}>
    <header>
      <h3>{isEditMode ? 'Edit Member' : 'Add Member'}</h3>
    </header>

    <section>
      <!-- Subject Type Selection -->
      <div data-field>
        <label>Member Type *</label>
        <OatSelect
          bind:value={memberForm.subject_type}
          options={[
            { value: 'user', label: 'User' },
            { value: 'group', label: 'Group' }
          ]}
          required
          disabled={loading || isEditMode}
          onchange={onSubjectTypeChange}
        />
        {#if isEditMode}
          <p class="text-lighter hint">Member type cannot be changed when editing.</p>
        {/if}
      </div>

      <!-- User/Group Selection -->
      <div data-field>
        <label>
          {memberForm.subject_type === 'user' ? 'User' : 'Group'} *
        </label>
        {#if isEditMode && memberData}
          <div class="selected-member hstack gap-2">
            <div class="icon-box">
              {#if memberData.subject_type === 'user'}
                <IconUser size={16} />
              {:else}
                <IconUsers size={16} />
              {/if}
            </div>
            <div>
              <div class="member-name">{memberData.subject_name}</div>
              <div class="text-lighter member-id">{memberData.subject_id}</div>
            </div>
          </div>
          <p class="text-lighter hint">Member cannot be changed when editing.</p>
        {:else}
          <UserGroupSelector
            bind:type={memberForm.subject_type}
            bind:selectedSubject={selectedSubject}
            placeholder="Search {memberForm.subject_type}s..."
            disabled={loading}
          />
        {/if}
      </div>

      <!-- Role Selection -->
      <div data-field>
        <label>Role *</label>
        <OatSelect
          bind:value={memberForm.role}
          options={[
            { value: 'user', label: 'User - Can view and trigger flows' },
            { value: 'operator', label: 'Operator - Can view and trigger accessible flows and see all executions' },
            { value: 'reviewer', label: 'Reviewer - Can approve flows and view all content' },
            { value: 'admin', label: 'Admin - Full access to namespace management' }
          ]}
          required
          disabled={loading}
        />
      </div>

      <!-- Prefix Access (edit mode, user/operator role only) -->
      {#if isEditMode && memberData && ['user', 'operator'].includes(memberData.role) && ['user', 'operator'].includes(memberForm.role)}
        <div>
          <p class="text-lighter hint mb-2">
            {memberForm.role === 'operator' ? 'Operators' : 'Users'} can only see ungrouped flows by default. Grant access to specific groups below.
          </p>

          {#if memberPrefixes.length > 0}
            <div class="prefix-list hstack gap-1 mb-2" style="flex-wrap:wrap">
              {#each memberPrefixes as prefix}
                <span class="prefix-tag hstack gap-1">
                  <IconFolder size={14} />
                  {prefix.prefix}
                  <button
                    type="button"
                    data-variant="danger"
                    class="prefix-remove"
                    onclick={() => removePrefix(prefix.prefix)}
                    disabled={prefixLoading}
                    title="Remove access"
                  >
                    <IconX size={14} />
                  </button>
                </span>
              {/each}
            </div>
          {/if}

          <FlowGroupSelector {namespace} bind:value={selectedPrefix} allowCreate={false} />
        </div>
      {/if}
    </section>

    <footer>
      <button
        type="button"
        data-variant="secondary"
        onclick={onClose}
        disabled={loading}
      >
        Cancel
      </button>
      <button
        type="submit"
        disabled={loading}
      >
        <span class="hstack gap-2 justify-center" aria-busy={loading} data-spinner="small">
          {isEditMode ? 'Update' : 'Add'}
        </span>
      </button>
    </footer>
  </form>
</dialog>

<style>
  dialog {
    max-width: 32rem;
    width: 100%;
  }
  .hint {
    font-size: var(--text-8);
    margin-top: var(--space-1);
  }
  .selected-member {
    padding: var(--space-3);
    background: var(--faint);
    border-radius: var(--radius-medium);
    border: 1px solid var(--border);
  }
  .member-name {
    font-size: var(--text-7);
    font-weight: var(--font-medium);
  }
  .member-id {
    font-size: var(--text-8);
  }
  .prefix-tag {
    padding: var(--space-1) var(--space-3);
    border-radius: var(--radius-medium);
    font-size: var(--text-7);
    background: var(--faint);
    color: var(--primary);
  }
  .prefix-remove {
    all: unset;
    cursor: pointer;
    display: inline-flex;
  }
</style>
