import type {
  User,
  UserWithGroups,
  UserProfileResponse,
  AuthReq,
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
  NodeReq,
  NodeResp,
  NodesPaginateResponse,
  NodeStatsResp,
  CredentialReq,
  CredentialResp,
  CredentialsPaginateResponse,
  FlowSecretReq,
  FlowSecretResp,
  NamespaceReq,
  NamespaceResp,
  NamespaceMemberReq,
  NamespaceMembersResponse,
  ApprovalActionReq,
  ApprovalActionResp,
  ApprovalsPaginateResponse,
  ExecutionsPaginateResponse,
  ExecutionSummary,
  UsersPaginateResponse,
  GroupsPaginateResponse,
  PaginateRequest,
  ApprovalPaginateRequest,
  GroupAccessReq,
  ExecutorConfigResponse,
  ApiErrorResponse,
  NamespacesPaginateResponse
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

async function baseFetch<T>(url: string, options: RequestInit = {}): Promise<T> {
  const response = await fetch(url, {
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
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
        baseFetch<NamespaceMembersResponse>(`/api/v1/${namespace}/members`),
      add: (namespace: string, member: NamespaceMemberReq) =>
        baseFetch<void>(`/api/v1/${namespace}/members`, {
          method: 'POST',
          body: JSON.stringify(member),
        }),
      update: (namespace: string, memberId: string, member: Partial<NamespaceMemberReq>) =>
        baseFetch<void>(`/api/v1/${namespace}/members/${memberId}`, {
          method: 'PUT',
          body: JSON.stringify(member),
        }),
      remove: (namespace: string, memberId: string) =>
        baseFetch<void>(`/api/v1/${namespace}/members/${memberId}`, {
          method: 'DELETE',
        }),
    },

    // Namespace group access
    groups: {
      list: (namespace: string) =>
        baseFetch<Group[]>(`/api/v1/${namespace}/groups`),
      add: (namespace: string, groupAccess: GroupAccessReq) =>
        baseFetch<void>(`/api/v1/${namespace}/groups`, {
          method: 'POST',
          body: JSON.stringify(groupAccess),
        }),
      remove: (namespace: string, groupId: string) =>
        baseFetch<void>(`/api/v1/${namespace}/groups/${groupId}`, {
          method: 'DELETE',
        }),
    },
  },

  // Flows
  flows: {
    list: (namespace: string, params: PaginateRequest = {}) =>
      baseFetch<FlowsPaginateResponse>(`/api/v1/${namespace}/flows${buildQueryString(params)}`),
    create: (namespace: string, flowData: FlowCreateReq) =>
      baseFetch<FlowCreateResp>(`/api/v1/${namespace}/flows`, {
        method: 'POST',
        body: JSON.stringify(flowData),
      }),
    getConfig: (namespace: string, flowId: string) =>
      baseFetch<FlowCreateReq>(`/api/v1/${namespace}/flows/${flowId}/config`),
    update: (namespace: string, flowId: string, flowData: FlowUpdateReq) =>
      baseFetch<FlowCreateResp>(`/api/v1/${namespace}/flows/${flowId}`, {
        method: 'PUT',
        body: JSON.stringify(flowData),
      }),
    delete: (namespace: string, flowId: string) =>
      baseFetch<void>(`/api/v1/${namespace}/flows/${flowId}`, {
        method: 'DELETE',
      }),
    getInputs: (namespace: string, flowId: string) =>
      baseFetch<FlowInputsResp>(`/api/v1/${namespace}/flows/${flowId}/inputs`),
    getMeta: (namespace: string, flowId: string) =>
      baseFetch<FlowMetaResp>(`/api/v1/${namespace}/flows/${flowId}/meta`),
    trigger: (namespace: string, flowId: string, inputs: Record<string, any>) => {
      const formData = new FormData();
      Object.entries(inputs).forEach(([key, value]) => {
        formData.append(key, String(value));
      });
      
      return baseFetch<FlowTriggerResp>(`/api/v1/${namespace}/trigger/${flowId}`, {
        method: 'POST',
        body: formData,
        headers: {},
      });
    },
  },

  // Nodes
  nodes: {
    list: (namespace: string, params: PaginateRequest = {}) =>
      baseFetch<NodesPaginateResponse>(`/api/v1/${namespace}/nodes${buildQueryString(params)}`),
    getStats: (namespace: string) =>
      baseFetch<NodeStatsResp>(`/api/v1/${namespace}/nodes/stats`),
    getById: (namespace: string, id: string) =>
      baseFetch<NodeResp>(`/api/v1/${namespace}/nodes/${id}`),
    create: (namespace: string, node: NodeReq) =>
      baseFetch<NodeResp>(`/api/v1/${namespace}/nodes`, {
        method: 'POST',
        body: JSON.stringify(node),
      }),
    update: (namespace: string, id: string, node: Partial<NodeReq>) =>
      baseFetch<NodeResp>(`/api/v1/${namespace}/nodes/${id}`, {
        method: 'PUT',
        body: JSON.stringify(node),
      }),
    delete: (namespace: string, id: string) =>
      baseFetch<void>(`/api/v1/${namespace}/nodes/${id}`, {
        method: 'DELETE',
      }),
  },

  // Credentials
  credentials: {
    list: (namespace: string, params: PaginateRequest = {}) =>
      baseFetch<CredentialsPaginateResponse>(`/api/v1/${namespace}/credentials${buildQueryString(params)}`),
    getById: (namespace: string, id: string) =>
      baseFetch<CredentialResp>(`/api/v1/${namespace}/credentials/${id}`),
    create: (namespace: string, credential: CredentialReq) =>
      baseFetch<CredentialResp>(`/api/v1/${namespace}/credentials`, {
        method: 'POST',
        body: JSON.stringify(credential),
      }),
    update: (namespace: string, id: string, credential: Partial<CredentialReq>) =>
      baseFetch<CredentialResp>(`/api/v1/${namespace}/credentials/${id}`, {
        method: 'PUT',
        body: JSON.stringify(credential),
      }),
    delete: (namespace: string, id: string) =>
      baseFetch<void>(`/api/v1/${namespace}/credentials/${id}`, {
        method: 'DELETE',
      }),
  },

  // Flow secrets
  flowSecrets: {
    list: (namespace: string, flowId: string) =>
      baseFetch<FlowSecretResp[]>(`/api/v1/${namespace}/flows/${flowId}/secrets`),
    getById: (namespace: string, flowId: string, secretId: string) =>
      baseFetch<FlowSecretResp>(`/api/v1/${namespace}/flows/${flowId}/secrets/${secretId}`),
    create: (namespace: string, flowId: string, secret: FlowSecretReq) =>
      baseFetch<FlowSecretResp>(`/api/v1/${namespace}/flows/${flowId}/secrets`, {
        method: 'POST',
        body: JSON.stringify(secret),
      }),
    update: (namespace: string, flowId: string, secretId: string, secret: Partial<FlowSecretReq>) =>
      baseFetch<FlowSecretResp>(`/api/v1/${namespace}/flows/${flowId}/secrets/${secretId}`, {
        method: 'PUT',
        body: JSON.stringify(secret),
      }),
    delete: (namespace: string, flowId: string, secretId: string) =>
      baseFetch<void>(`/api/v1/${namespace}/flows/${flowId}/secrets/${secretId}`, {
        method: 'DELETE',
      }),
  },

  // Approvals
  approvals: {
    list: (namespace: string, params: ApprovalPaginateRequest = {}) =>
      baseFetch<ApprovalsPaginateResponse>(`/api/v1/${namespace}/approvals${buildQueryString(params)}`),
    get: (namespace: string, approvalId: string) =>
      baseFetch<ApprovalDetailsResp>(`/api/v1/${namespace}/approvals/${approvalId}`),
    action: (namespace: string, approvalId: string, action: ApprovalActionReq) =>
      baseFetch<ApprovalActionResp>(`/api/v1/${namespace}/approvals/${approvalId}`, {
        method: 'POST',
        body: JSON.stringify(action),
      }),
  },

  // Executions/History
  executions: {
    list: (namespace: string, params: PaginateRequest = {}) =>
      baseFetch<ExecutionsPaginateResponse>(`/api/v1/${namespace}/flows/executions${buildQueryString(params)}`),
    getById: (namespace: string, execId: string) =>
      baseFetch<ExecutionSummary>(`/api/v1/${namespace}/flows/executions/${execId}`),
    listForFlow: (namespace: string, flowId: string, params: PaginateRequest = {}) =>
      baseFetch<ExecutionsPaginateResponse>(`/api/v1/${namespace}/flows/${flowId}/executions${buildQueryString(params)}`),
    cancel: (namespace: string, execId: string) =>
      baseFetch<{message: string; execID: string}>(`/api/v1/${namespace}/flows/executions/${execId}/cancel`, {
        method: 'POST',
      }),
  },

  // Executors
  executors: {
    list: () => baseFetch<{executors: string[]}>('/api/v1/executors'),
    getConfig: (executor: string) =>
      baseFetch<ExecutorConfigResponse>(`/api/v1/executors/${executor}/config`),
  },

  // Utility endpoints
  ping: () => baseFetch<string>('/ping'),
};