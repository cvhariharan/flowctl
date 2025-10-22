<script lang="ts">
  import { handleInlineError } from '$lib/utils/errorHandling';
  import UserGroupSelector from '$lib/components/shared/UserGroupSelector.svelte';
  import type { NamespaceMemberReq, NamespaceMemberResp, User, Group } from '$lib/types';
  
  interface Props {
    isEditMode?: boolean;
    memberData?: NamespaceMemberResp | null;
    onSave: (memberData: NamespaceMemberReq) => void;
    onClose: () => void;
  }

  let {
    isEditMode = false,
    memberData = null,
    onSave,
    onClose
  }: Props = $props();


  // Form state
  let memberForm = $state<NamespaceMemberReq>({
    subject_type: 'user',
    subject_id: '',
    role: 'user'
  });
  
  let selectedSubject = $state<User | Group | null>(null);
  let loading = $state(false);

  // Initialize form data when memberData changes
  $effect(() => {
    if (isEditMode && memberData) {
      // Pre-populate form with existing member data
      memberForm.subject_type = memberData.subject_type as 'user' | 'group';
      memberForm.subject_id = memberData.subject_id;
      memberForm.role = memberData.role as 'user' | 'reviewer' | 'admin';

      // Create a selectedSubject object for the UserGroupSelector
      selectedSubject = {
        id: memberData.subject_id,
        name: memberData.subject_name,
        username: memberData.subject_name // For users, this would be the username
      } as User | Group;
    } else {
      resetForm();
    }
  });

  // Update subject_id when selectedSubject changes
  $effect(() => {
    memberForm.subject_id = selectedSubject?.id || '';
  });

  function onSubjectTypeChange() {
    selectedSubject = null;
    memberForm.subject_id = '';
  }

  async function handleSubmit() {
    try {
      loading = true;
      
      // Basic client-side validation
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

  function handleClose() {
    onClose();
  }

  function resetForm() {
    memberForm = {
      subject_type: 'user',
      subject_id: '',
      role: 'user'
    };
    selectedSubject = null;
  }

  // Close on Escape key
  function handleKeydown(event: KeyboardEvent) {
    if (event.key === 'Escape') {
      handleClose();
    }
  }
</script>

<svelte:window onkeydown={handleKeydown} />

<!-- Modal Backdrop -->
<div class="fixed inset-0 z-50 flex items-center justify-center bg-gray-900/60 p-4" onclick={handleClose} onkeydown={(e) => e.key === 'Escape' && handleClose()} role="dialog" aria-modal="true" tabindex="-1">
  <!-- Modal Content -->
  <div class="bg-white rounded-lg shadow-lg w-full max-w-lg max-h-[90vh] overflow-y-auto" onclick={(e) => e.stopPropagation()}>
      <div class="p-6">
        <h3 class="font-bold text-lg mb-4 text-gray-900">
          {isEditMode ? 'Edit Member' : 'Add Member'}
        </h3>
        

        <form onsubmit={(e) => { e.preventDefault(); handleSubmit(); }}>
          <!-- Subject Type Selection -->
          <div class="mb-4">
            <label class="block mb-1 font-medium text-gray-900">Member Type *</label>
            <select
              bind:value={memberForm.subject_type}
              onchange={onSubjectTypeChange}
              class="bg-gray-50 border border-gray-300 text-gray-900 text-sm rounded-lg focus:ring-primary-500 focus:border-primary-500 block w-full p-2.5"
              required
              disabled={loading || isEditMode}
            >
              <option value="user">User</option>
              <option value="group">Group</option>
            </select>
            {#if isEditMode}
              <p class="text-xs text-gray-500 mt-1">Member type cannot be changed when editing.</p>
            {/if}
          </div>

          <!-- User/Group Selection -->
          <div class="mb-4">
            <label class="block mb-1 font-medium text-gray-900">
              {memberForm.subject_type === 'user' ? 'User' : 'Group'} *
            </label>
            {#if isEditMode && memberData}
              <!-- Show selected member in edit mode -->
              <div class="p-3 bg-gray-50 rounded-lg border">
                <div class="flex items-center">
                  <div class="w-8 h-8 rounded-lg flex items-center justify-center mr-3 bg-primary-100">
                    <svg class="w-4 h-4 text-primary-600"
                         fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      {#if memberData.subject_type === 'user'}
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"></path>
                      {:else}
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z"></path>
                      {/if}
                    </svg>
                  </div>
                  <div>
                    <div class="text-sm font-medium text-gray-900">{memberData.subject_name}</div>
                    <div class="text-xs text-gray-500">{memberData.subject_id}</div>
                  </div>
                </div>
              </div>
              <p class="text-xs text-gray-500 mt-1">Member cannot be changed when editing.</p>
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
          <div class="mb-4">
            <label class="block mb-1 font-medium text-gray-900">Role *</label>
            <select
              bind:value={memberForm.role}
              class="bg-gray-50 border border-gray-300 text-gray-900 text-sm rounded-lg focus:ring-primary-500 focus:border-primary-500 block w-full p-2.5"
              required
              disabled={loading}
            >
              <option value="user">User - Can view and trigger flows</option>
              <option value="reviewer">Reviewer - Can approve flows and view all content</option>
              <option value="admin">Admin - Full access to namespace management</option>
            </select>
          </div>

          <!-- Actions -->
          <div class="flex justify-end gap-2 mt-6">
            <button
              type="button"
              onclick={handleClose}
              class="inline-flex items-center px-5 py-2.5 text-sm font-medium text-gray-700 bg-gray-100 rounded-lg hover:bg-gray-200 disabled:opacity-50 cursor-pointer"
              disabled={loading}
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={loading}
              class="inline-flex items-center px-5 py-2.5 text-sm font-medium text-white bg-primary-500 rounded-lg hover:bg-primary-600 focus:ring-4 focus:outline-none focus:ring-primary-300 disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer"
            >
              {#if loading}
                <svg class="animate-spin -ml-1 mr-2 h-4 w-4 text-white" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                  <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                  <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                </svg>
              {/if}
              {isEditMode ? 'Update Member' : 'Add Member'}
            </button>
          </div>
        </form>
      </div>
    </div>
</div>