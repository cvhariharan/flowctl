import type {
  User,
  UserWithGroups,
  UserProfileResponse,
  AuthReq,
  SSOProvider,
  Group,
  GroupWithUsers,
  FlowListResponse,
  FlowsPaginateResponse,
  FlowInputsResp,
  FlowMetaResp,
  FlowTriggerResp,
  FlowCreateReq,
  FlowCreateResp,
  FlowUpdateReq,
  Flow,
  FlowGroupsResponse,
  FlowGroupResp,
  FlowGroupDetail,
  NodeReq,
  NodeResp,
  NodesPaginateResponse,
  NodeStatsResp,
  CredentialReq,
  CredentialResp,
  CredentialsPaginateResponse,
  FlowSecretReq,
  FlowSecretUpdateReq,
  FlowSecretResp,
  NamespaceSecretReq,
  NamespaceSecretUpdateReq,
  NamespaceSecretResp,
  NamespaceReq,
  NamespaceResp,
  NamespaceMemberReq,
  NamespaceMembersResponse,
  ApprovalActionReq,
  ApprovalActionResp,
  ApprovalDetailsResp,
  ApprovalsPaginateResponse,
  ExecutionsPaginateResponse,
  ExecutionSummary,
  UsersPaginateResponse,
  GroupsPaginateResponse,
  PaginateRequest,
  NodePaginateRequest,
  ApprovalPaginateRequest,
  GroupAccessReq,
  ExecutorConfigResponse,
  ExecutorsListResponse,
  ApiErrorResponse,
  NamespacesPaginateResponse,
  UserSchedule,
  ScheduleCreateReq,
  ScheduleUpdateReq,
  SchedulesPaginateResponse,
  ApiToken,
  ApiTokenCreated,
  AppInfoResponse,
  ArtifactMetadata
} from './types.js';

export class ApiError extends Error {
  constructor(
    public status: number,
    public statusText: string,
    public data?: ApiErrorResponse
  ) {
    super(`API Error: ${status} ${statusText}`);
    this.name = 'ApiError';
  }
}

export function getCsrfToken(): string {
  const match = document.cookie.match(/(?:^|;\s*)_csrf=([^;]+)/);
  return match ? decodeURIComponent(match[1]) : '';
}

async function baseFetch<T>(url: string, options: RequestInit = {}): Promise<T> {
  const method = (options.method ?? 'GET').toUpperCase();
  const csrfHeaders: Record<string, string> =
    method !== 'GET' && method !== 'HEAD'
      ? { 'X-CSRF-Token': getCsrfToken() }
      : {};

  const response = await fetch(url, {
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      ...csrfHeaders,
      ...options.headers,
    },
    ...options,
  });

  if (!response.ok) {
    let errorData: ApiErrorResponse | undefined;
    try {
      errorData = await response.json();
    } catch {
      // Ignore JSON parsing errors for error responses
    }
    throw new ApiError(response.status, response.statusText, errorData);
  }

  // Handle empty responses (e.g., 204 No Content)
  if (response.status === 204) {
    return {} as T;
  }

  const contentType = response.headers.get('content-type');
  if (contentType && contentType.includes('application/json')) {
    return response.json();
  }

  // Return response as text for non-JSON responses
  return response.text() as T;
}

function buildQueryString(params: Record<string, any>): string {
  const searchParams = new URLSearchParams();

  Object.entries(params).forEach(([key, value]) => {
    if (value !== undefined && value !== null && value !== '') {
      searchParams.append(key, String(value));
    }
  });

  const queryString = searchParams.toString();
  return queryString ? `?${queryString}` : '';
}

export const apiClient = {
  // Authentication
  auth: {
    login: (credentials: AuthReq) =>
      baseFetch<void>('/login', {
        method: 'POST',
        body: JSON.stringify(credentials),
      }),
    logout: () =>
      baseFetch<void>('/logout', {
        method: 'POST',
      }),
    getSSOProviders: () => baseFetch<SSOProvider[]>('/sso-providers'),
  },

  info: {
    get: () => baseFetch<AppInfoResponse>('/info'),
  },

  // Users
  users: {
    getProfile: () => baseFetch<UserProfileResponse>('/api/v1/users/profile'),
    list: (params: PaginateRequest = {}) =>
      baseFetch<UsersPaginateResponse>(`/api/v1/users${buildQueryString(params)}`),
    getById: (id: string) => baseFetch<UserWithGroups>(`/api/v1/users/${id}`),
    create: (user: Partial<User>) =>
      baseFetch<User>('/api/v1/users', {
        method: 'POST',
        body: JSON.stringify(user),
      }),
    update: (id: string, user: Partial<User>) =>
      baseFetch<User>(`/api/v1/users/${id}`, {
        method: 'PUT',
        body: JSON.stringify(user),
      }),
    delete: (id: string) =>
      baseFetch<void>(`/api/v1/users/${id}`, {
        method: 'DELETE',
      }),
    changePassword: (oldPassword: string, newPassword: string) =>
      baseFetch<{ message: string }>('/api/v1/users/change-password', {
        method: 'POST',
        body: JSON.stringify({ old_password: oldPassword, new_password: newPassword }),
      }),
  },

  // User API tokens
  apiTokens: {
    list: () => baseFetch<ApiToken[]>('/api/v1/users/me/api-tokens'),
    create: (label: string) =>
      baseFetch<ApiTokenCreated>('/api/v1/users/me/api-tokens', {
        method: 'POST',
        body: JSON.stringify({ label }),
      }),
    revoke: (uuid: string) =>
      baseFetch<void>(`/api/v1/users/me/api-tokens/${uuid}`, {
        method: 'DELETE',
      }),
  },

  // Groups
  groups: {
    list: (params: PaginateRequest = {}) =>
      baseFetch<GroupsPaginateResponse>(`/api/v1/groups${buildQueryString(params)}`),
    getById: (id: string) => baseFetch<GroupWithUsers>(`/api/v1/groups/${id}`),
    create: (group: Partial<Group>) =>
      baseFetch<Group>('/api/v1/groups', {
        method: 'POST',
        body: JSON.stringify(group),
      }),
    update: (id: string, group: Partial<Group>) =>
      baseFetch<Group>(`/api/v1/groups/${id}`, {
        method: 'PUT',
        body: JSON.stringify(group),
      }),
    delete: (id: string) =>
      baseFetch<void>(`/api/v1/groups/${id}`, {
        method: 'DELETE',
      }),
  },

  // Namespaces
  namespaces: {
    list: (params: PaginateRequest = {}) =>
      baseFetch<NamespacesPaginateResponse>(`/api/v1/namespaces${buildQueryString(params)}`),
    getById: (id: string) => baseFetch<NamespaceResp>(`/api/v1/namespaces/${id}`),
    create: (namespace: NamespaceReq) =>
      baseFetch<NamespaceResp>('/api/v1/namespaces', {
        method: 'POST',
        body: JSON.stringify(namespace),
      }),
    update: (id: string, namespace: NamespaceReq) =>
      baseFetch<NamespaceResp>(`/api/v1/namespaces/${id}`, {
        method: 'PUT',
        body: JSON.stringify(namespace),
      }),
    delete: (id: string) =>
      baseFetch<void>(`/api/v1/namespaces/${id}`, {
        method: 'DELETE',
      }),

    // Namespace members
    members: {
      list: (namespace: string) =>
        baseFetch<NamespaceMembersResponse>(`/api/v1/${encodeURIComponent(namespace)}/members`),
      add: (namespace: string, member: NamespaceMemberReq) =>
        baseFetch<void>(`/api/v1/${encodeURIComponent(namespace)}/members`, {
          method: 'POST',
          body: JSON.stringify(member),
        }),
      update: (namespace: string, memberId: string, member: Partial<NamespaceMemberReq>) =>
        baseFetch<void>(`/api/v1/${encodeURIComponent(namespace)}/members/${memberId}`, {
          method: 'PUT',
          body: JSON.stringify(member),
        }),
      remove: (namespace: string, memberId: string) =>
        baseFetch<void>(`/api/v1/${encodeURIComponent(namespace)}/members/${memberId}`, {
          method: 'DELETE',
        }),
      groups: {
        list: (namespace: string, memberId: string) =>
          baseFetch<FlowGroupsResponse>(`/api/v1/${encodeURIComponent(namespace)}/members/${memberId}/groups`),
        add: (namespace: string, memberId: string, data: { prefix: string }) =>
          baseFetch<void>(`/api/v1/${encodeURIComponent(namespace)}/members/${memberId}/groups`, {
            method: 'POST',
            body: JSON.stringify(data),
          }),
        remove: (namespace: string, memberId: string, group: string) =>
          baseFetch<void>(`/api/v1/${encodeURIComponent(namespace)}/members/${memberId}/groups/${group}`, {
            method: 'DELETE',
          }),
      },
    },

    // Namespace group access
    groups: {
      list: (namespace: string) =>
        baseFetch<Group[]>(`/api/v1/${encodeURIComponent(namespace)}/groups`),
      add: (namespace: string, groupAccess: GroupAccessReq) =>
        baseFetch<void>(`/api/v1/${encodeURIComponent(namespace)}/groups`, {
          method: 'POST',
          body: JSON.stringify(groupAccess),
        }),
      remove: (namespace: string, groupId: string) =>
        baseFetch<void>(`/api/v1/${encodeURIComponent(namespace)}/groups/${groupId}`, {
          method: 'DELETE',
        }),
    },
  },

  // Flows
  flows: {
    list: (namespace: string, params: PaginateRequest = {}) =>
      baseFetch<FlowsPaginateResponse>(`/api/v1/${encodeURIComponent(namespace)}/flows${buildQueryString(params)}`),
    create: (namespace: string, flowData: FlowCreateReq) =>
      baseFetch<FlowCreateResp>(`/api/v1/${encodeURIComponent(namespace)}/flows`, {
        method: 'POST',
        body: JSON.stringify(flowData),
      }),
    getConfig: (namespace: string, flowId: string) =>
      baseFetch<FlowCreateReq>(`/api/v1/${encodeURIComponent(namespace)}/flows/${flowId}/config`),
    update: (namespace: string, flowId: string, flowData: FlowUpdateReq) =>
      baseFetch<FlowCreateResp>(`/api/v1/${encodeURIComponent(namespace)}/flows/${flowId}`, {
        method: 'PUT',
        body: JSON.stringify(flowData),
      }),
    delete: (namespace: string, flowId: string) =>
      baseFetch<void>(`/api/v1/${encodeURIComponent(namespace)}/flows/${flowId}`, {
        method: 'DELETE',
      }),
    getInputs: (namespace: string, flowId: string) =>
      baseFetch<FlowInputsResp>(`/api/v1/${encodeURIComponent(namespace)}/flows/${flowId}/inputs`),
    getMeta: (namespace: string, flowId: string) =>
      baseFetch<FlowMetaResp>(`/api/v1/${encodeURIComponent(namespace)}/flows/${flowId}/meta`),
    trigger: (namespace: string, flowId: string, inputs: Record<string, any>) => {
      const formData = new FormData();
      Object.entries(inputs).forEach(([key, value]) => {
        formData.append(key, String(value));
      });

      return baseFetch<FlowTriggerResp>(`/api/v1/${encodeURIComponent(namespace)}/trigger/${flowId}`, {
        method: 'POST',
        body: formData,
        // No Content-Type: browser sets it automatically with the correct multipart boundary.
        // CSRF token is still required and added here explicitly.
        headers: { 'X-CSRF-Token': getCsrfToken() },
      });
    },
    groups: {
      me: (namespace: string) =>
        baseFetch<FlowGroupsResponse>(`/api/v1/${encodeURIComponent(namespace)}/flows/groups/me`),
      get: (namespace: string, group: string) =>
        baseFetch<FlowListResponse>(`/api/v1/${encodeURIComponent(namespace)}/flows/groups/${group}`),
      list: (namespace: string) =>
        baseFetch<FlowGroupsResponse>(`/api/v1/${encodeURIComponent(namespace)}/flows/groups`),
      create: (namespace: string, data: { name: string; description: string }) =>
        baseFetch<FlowGroupDetail>(`/api/v1/${encodeURIComponent(namespace)}/flows/groups`, {
          method: 'POST',
          body: JSON.stringify(data),
        }),
      update: (namespace: string, groupId: string, data: { name: string; description: string }) =>
        baseFetch<FlowGroupDetail>(`/api/v1/${encodeURIComponent(namespace)}/flows/groups/${groupId}`, {
          method: 'PUT',
          body: JSON.stringify(data),
        }),
      delete: (namespace: string, groupId: string) =>
        baseFetch<void>(`/api/v1/${encodeURIComponent(namespace)}/flows/groups/${groupId}`, {
          method: 'DELETE',
        }),
    },
    schedules: {
      list: (namespace: string, flowId: string, params: PaginateRequest = {}) =>
        baseFetch<SchedulesPaginateResponse>(
          `/api/v1/${encodeURIComponent(namespace)}/flows/${flowId}/schedules${buildQueryString(params)}`
        ),
      create: (namespace: string, flowId: string, schedule: ScheduleCreateReq) =>
        baseFetch<{ schedule_id: string }>(
          `/api/v1/${encodeURIComponent(namespace)}/flows/${flowId}/schedules`,
          { method: 'POST', body: JSON.stringify(schedule) }
        ),
      update: (namespace: string, flowId: string, scheduleId: string, schedule: ScheduleUpdateReq) =>
        baseFetch<{ schedule_id: string }>(
          `/api/v1/${encodeURIComponent(namespace)}/flows/${flowId}/schedules/${scheduleId}`,
          { method: 'PUT', body: JSON.stringify(schedule) }
        ),
      delete: (namespace: string, flowId: string, scheduleId: string) =>
        baseFetch<void>(
          `/api/v1/${encodeURIComponent(namespace)}/flows/${flowId}/schedules/${scheduleId}`,
          { method: 'DELETE' }
        ),
    },
  },

  // Nodes
  nodes: {
    list: (namespace: string, params: NodePaginateRequest = {}) =>
      baseFetch<NodesPaginateResponse>(`/api/v1/${encodeURIComponent(namespace)}/nodes${buildQueryString(params)}`),
    getStats: (namespace: string) =>
      baseFetch<NodeStatsResp>(`/api/v1/${encodeURIComponent(namespace)}/nodes/stats`),
    getById: (namespace: string, id: string) =>
      baseFetch<NodeResp>(`/api/v1/${encodeURIComponent(namespace)}/nodes/${id}`),
    create: (namespace: string, node: NodeReq) =>
      baseFetch<NodeResp>(`/api/v1/${encodeURIComponent(namespace)}/nodes`, {
        method: 'POST',
        body: JSON.stringify(node),
      }),
    update: (namespace: string, id: string, node: Partial<NodeReq>) =>
      baseFetch<NodeResp>(`/api/v1/${encodeURIComponent(namespace)}/nodes/${id}`, {
        method: 'PUT',
        body: JSON.stringify(node),
      }),
    delete: (namespace: string, id: string) =>
      baseFetch<void>(`/api/v1/${encodeURIComponent(namespace)}/nodes/${id}`, {
        method: 'DELETE',
      }),
  },

  // Credentials
  credentials: {
    list: (namespace: string, params: PaginateRequest = {}) =>
      baseFetch<CredentialsPaginateResponse>(`/api/v1/${encodeURIComponent(namespace)}/credentials${buildQueryString(params)}`),
    getById: (namespace: string, id: string) =>
      baseFetch<CredentialResp>(`/api/v1/${encodeURIComponent(namespace)}/credentials/${id}`),
    create: (namespace: string, credential: CredentialReq) =>
      baseFetch<CredentialResp>(`/api/v1/${encodeURIComponent(namespace)}/credentials`, {
        method: 'POST',
        body: JSON.stringify(credential),
      }),
    update: (namespace: string, id: string, credential: Partial<CredentialReq>) =>
      baseFetch<CredentialResp>(`/api/v1/${encodeURIComponent(namespace)}/credentials/${id}`, {
        method: 'PUT',
        body: JSON.stringify(credential),
      }),
    delete: (namespace: string, id: string) =>
      baseFetch<void>(`/api/v1/${encodeURIComponent(namespace)}/credentials/${id}`, {
        method: 'DELETE',
      }),
  },

  // Flow secrets
  flowSecrets: {
    list: (namespace: string, flowId: string) =>
      baseFetch<FlowSecretResp[]>(`/api/v1/${encodeURIComponent(namespace)}/flows/${flowId}/secrets`),
    getById: (namespace: string, flowId: string, secretId: string) =>
      baseFetch<FlowSecretResp>(`/api/v1/${encodeURIComponent(namespace)}/flows/${flowId}/secrets/${secretId}`),
    create: (namespace: string, flowId: string, secret: FlowSecretReq) =>
      baseFetch<FlowSecretResp>(`/api/v1/${encodeURIComponent(namespace)}/flows/${flowId}/secrets`, {
        method: 'POST',
        body: JSON.stringify(secret),
      }),
    update: (namespace: string, flowId: string, secretId: string, secret: FlowSecretUpdateReq) =>
      baseFetch<FlowSecretResp>(`/api/v1/${encodeURIComponent(namespace)}/flows/${flowId}/secrets/${secretId}`, {
        method: 'PUT',
        body: JSON.stringify(secret),
      }),
    delete: (namespace: string, flowId: string, secretId: string) =>
      baseFetch<void>(`/api/v1/${encodeURIComponent(namespace)}/flows/${flowId}/secrets/${secretId}`, {
        method: 'DELETE',
      }),
  },

  // Namespace secrets
  namespaceSecrets: {
    list: (namespace: string) =>
      baseFetch<NamespaceSecretResp[]>(`/api/v1/${encodeURIComponent(namespace)}/secrets`),
    getById: (namespace: string, secretId: string) =>
      baseFetch<NamespaceSecretResp>(`/api/v1/${encodeURIComponent(namespace)}/secrets/${secretId}`),
    create: (namespace: string, secret: NamespaceSecretReq) =>
      baseFetch<NamespaceSecretResp>(`/api/v1/${encodeURIComponent(namespace)}/secrets`, {
        method: 'POST',
        body: JSON.stringify(secret),
      }),
    update: (namespace: string, secretId: string, secret: NamespaceSecretUpdateReq) =>
      baseFetch<NamespaceSecretResp>(`/api/v1/${encodeURIComponent(namespace)}/secrets/${secretId}`, {
        method: 'PUT',
        body: JSON.stringify(secret),
      }),
    delete: (namespace: string, secretId: string) =>
      baseFetch<void>(`/api/v1/${encodeURIComponent(namespace)}/secrets/${secretId}`, {
        method: 'DELETE',
      }),
  },

  // Approvals
  approvals: {
    list: (namespace: string, params: ApprovalPaginateRequest = {}) =>
      baseFetch<ApprovalsPaginateResponse>(`/api/v1/${encodeURIComponent(namespace)}/approvals${buildQueryString(params)}`),
    get: (namespace: string, approvalId: string) =>
      baseFetch<ApprovalDetailsResp>(`/api/v1/${encodeURIComponent(namespace)}/approvals/${approvalId}`),
    action: (namespace: string, approvalId: string, action: ApprovalActionReq) =>
      baseFetch<ApprovalActionResp>(`/api/v1/${encodeURIComponent(namespace)}/approvals/${approvalId}`, {
        method: 'POST',
        body: JSON.stringify(action),
      }),
  },

  // Executions/History
  executions: {
    list: (namespace: string, params: PaginateRequest = {}) =>
      baseFetch<ExecutionsPaginateResponse>(`/api/v1/${encodeURIComponent(namespace)}/flows/executions${buildQueryString(params)}`),
    getById: (namespace: string, execId: string) =>
      baseFetch<ExecutionSummary>(`/api/v1/${encodeURIComponent(namespace)}/flows/executions/${execId}`),
    listForFlow: (namespace: string, flowId: string, params: PaginateRequest = {}) =>
      baseFetch<ExecutionsPaginateResponse>(`/api/v1/${encodeURIComponent(namespace)}/flows/${flowId}/executions${buildQueryString(params)}`),
    cancel: (namespace: string, execId: string) =>
      baseFetch<{message: string; execID: string}>(`/api/v1/${encodeURIComponent(namespace)}/flows/executions/${execId}/cancel`, {
        method: 'POST',
      }),
    retry: (namespace: string, execId: string) =>
      baseFetch<void>(`/api/v1/${encodeURIComponent(namespace)}/flows/executions/${execId}/retry`, {
        method: 'POST',
      }),
    getArtifacts: (namespace: string, execId: string) =>
      baseFetch<ArtifactMetadata[]>(`/api/v1/${encodeURIComponent(namespace)}/flows/executions/${execId}/artifacts`),
  },

  // Executors
  executors: {
    list: () => baseFetch<ExecutorsListResponse>('/api/v1/executors'),
    getConfig: (executor: string) =>
      baseFetch<ExecutorConfigResponse>(`/api/v1/executors/${executor}/config`),
  },

  // Messengers
  messengers: {
    list: () => baseFetch<Record<string, any>>('/api/v1/messengers'),
  },

  // Permissions
  permissions: {
    check: (permissions: { resource: string; action: string; domain: string }[]) =>
      baseFetch<{ results: Record<string, boolean> }>('/api/v1/permissions/check', {
        method: 'POST',
        body: JSON.stringify({ permissions }),
      }),
  },

  // Utility endpoints
  ping: () => baseFetch<string>('/ping'),
};
