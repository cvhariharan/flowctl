<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import UserGroupSelector from '$lib/components/shared/UserGroupSelector.svelte';
  import type { NamespaceMemberReq, User, Group } from '$lib/types';
  
  interface Props {
    show?: boolean;
    onSave: (memberData: NamespaceMemberReq) => void;
    onClose: () => void;
  }

  let {
    show = false,
    onSave,
    onClose
  }: Props = $props();

  const dispatch = createEventDispatcher<{
    save: NamespaceMemberReq;
    close: void;
  }>();

  // Form state
  let memberForm = $state<NamespaceMemberReq>({
    subject_type: 'user',
    subject_id: '',
    role: 'user'
  });
  
  let selectedSubject = $state<User | Group | null>(null);
  let loading = $state(false);
  let error = $state('');

  // Reset form when show changes
  $effect(() => {
    if (show) {
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

  function handleSubmit() {
    try {
      loading = true;
      error = '';

      if (!selectedSubject || !memberForm.role) {
        error = 'Please select a member and role';
        return;
      }

      // Emit save event and call onSave prop
      dispatch('save', memberForm);
      onSave(memberForm);
    } catch (err) {
      console.error('Failed to save member:', err);
      error = 'Failed to save member';
    } finally {
      loading = false;
    }
  }

  function handleClose() {
    dispatch('close');
    onClose();
  }

  function resetForm() {
    memberForm = {
      subject_type: 'user',
      subject_id: '',
      role: 'user'
    };
    selectedSubject = null;
    error = '';
  }

  // Close on Escape key
  function handleKeydown(event: KeyboardEvent) {
    if (event.key === 'Escape') {
      handleClose();
    }
  }
</script>

<svelte:window on:keydown={handleKeydown} />

{#if show}
  <!-- Modal Backdrop -->
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-gray-900/60 p-4" on:click={handleClose}>
    <!-- Modal Content -->
    <div class="bg-white rounded-lg shadow-lg w-full max-w-lg max-h-[90vh] overflow-y-auto" on:click|stopPropagation>
      <div class="p-6">
        <h3 class="font-bold text-lg mb-4 text-gray-900">Add Member</h3>
        
        {#if error}
          <div class="mb-4 p-3 bg-red-50 border border-red-200 rounded-md">
            <p class="text-sm text-red-600">{error}</p>
          </div>
        {/if}
        
        <form on:submit|preventDefault={handleSubmit}>
          <!-- Subject Type Selection -->
          <div class="mb-4">
            <label class="block mb-1 font-medium text-gray-900">Member Type *</label>
            <select 
              bind:value={memberForm.subject_type}
              on:change={onSubjectTypeChange}
              class="bg-gray-50 border border-gray-300 text-gray-900 text-sm rounded-lg focus:ring-blue-500 focus:border-blue-500 block w-full p-2.5"
              required
              disabled={loading}
            >
              <option value="user">User</option>
              <option value="group">Group</option>
            </select>
          </div>

          <!-- User/Group Selection -->
          <div class="mb-4">
            <label class="block mb-1 font-medium text-gray-900">
              {memberForm.subject_type === 'user' ? 'User' : 'Group'} *
            </label>
            <UserGroupSelector 
              bind:type={memberForm.subject_type}
              bind:selectedSubject={selectedSubject}
              placeholder="Search {memberForm.subject_type}s..."
              disabled={loading}
            />
          </div>

          <!-- Role Selection -->
          <div class="mb-4">
            <label class="block mb-1 font-medium text-gray-900">Role *</label>
            <select 
              bind:value={memberForm.role}
              class="bg-gray-50 border border-gray-300 text-gray-900 text-sm rounded-lg focus:ring-blue-500 focus:border-blue-500 block w-full p-2.5"
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
              on:click={handleClose}
              class="inline-flex items-center px-5 py-2.5 text-sm font-medium text-gray-700 bg-gray-100 rounded-lg hover:bg-gray-200 disabled:opacity-50"
              disabled={loading}
            >
              Cancel
            </button>
            <button 
              type="submit"
              disabled={!selectedSubject || !memberForm.role || loading}
              class="inline-flex items-center px-5 py-2.5 text-sm font-medium text-white bg-blue-700 rounded-lg hover:bg-blue-800 focus:ring-4 focus:outline-none focus:ring-blue-300 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {#if loading}
                <svg class="animate-spin -ml-1 mr-2 h-4 w-4 text-white" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                  <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                  <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                </svg>
              {/if}
              Add Member
            </button>
          </div>
        </form>
      </div>
    </div>
  </div>
{/if}