// User and authentication types
export interface User {
  id: string;
  username: string;
  name: string;
  login_type: string;
  role: string;
}

export interface UserWithGroups extends User {
  groups: Group[];
}

export interface UserProfileResponse {
  id: string;
  username: string;
  name: string;
  role: string;
}

export interface AuthReq {
  username: string;
  password: string;
}

// Group types
export interface Group {
  id: string;
  name: string;
  description: string;
  users: User[];
}

export interface GroupWithUsers extends Group {
  users: User[];
}

// Flow types
export interface Flow {
  metadata: FlowMeta;
  inputs: FlowInput[];
  actions: FlowAction[];
}

export interface FlowListItem {
  id: string;
  slug: string;
  name: string;
  description: string;
  schedules: string[];
  step_count: number;
}

export interface FlowInput {
  name: string;
  label: string;
  description: string;
  required: boolean;
  type:
    | "string"
    | "number"
    | "password"
    | "file"
    | "datetime"
    | "checkbox"
    | "select";
  options: string[];
}

export interface FlowInputsResp {
  inputs: FlowInput[];
}

export interface FlowMeta {
  id: string;
  name: string;
  description: string;
  schedules: string[];
  namespace: string;
}

export interface FlowAction {
  id: string;
  name: string;
  executor: string;
  approval: boolean;
  on: string[];
}

export interface FlowMetaResp {
  meta: FlowMeta;
  actions: FlowAction[];
}

export interface FlowListResponse {
  flows: FlowListItem[];
}

export interface FlowTriggerResp {
  exec_id: string;
}

export interface FlowLogResp {
  action_id: string;
  message_type: "log" | "error" | "result" | "approval";
  value: string;
  results?: Record<string, string>;
}

// Node types
export interface NodeAuth {
  method: "private_key" | "password";
  credential_id: string;
}

export interface NodeReq {
  name: string;
  hostname: string;
  port: number;
  username: string;
  connection_type: "ssh" | "qssh";
  tags: string[];
  auth: NodeAuth;
}

export interface NodeResp {
  id: string;
  name: string;
  hostname: string;
  port: number;
  username: string;
  os_family: string;
  connection_type: string;
  tags: string[];
  auth: NodeAuth;
}

export interface NodeStatsResp {
  total_hosts: number;
  ssh_hosts: number;
  qssh_hosts: number;
}

// Credential types
export interface CredentialReq {
  name: string;
  key_type: "private_key" | "password";
  key_data: string;
}

export interface CredentialResp {
  id: string;
  name: string;
  key_type: string;
  last_accessed: string;
}

// Namespace types
export interface NamespaceReq {
  name: string;
}

export interface Namespace {
  id: string;
  name: string;
}

export interface NamespaceResp {
  id: string;
  name: string;
}

export interface NamespacesPaginateResponse
  extends PaginatedResponse<NamespaceResp> {
  namespaces: NamespaceResp[];
  page_count: number;
  total_count: number;
}

export interface NamespaceMemberReq {
  subject_id: string;
  subject_type: "user" | "group";
  role: "user" | "reviewer" | "admin";
}

export interface NamespaceMemberResp {
  id: string;
  subject_id: string;
  subject_name: string;
  subject_type: string;
  role: string;
  created_at: string;
  updated_at: string;
}

export interface NamespaceMembersResponse {
  members: NamespaceMemberResp[];
}

// Approval types
export interface ApprovalActionReq {
  action: string;
}

export interface ApprovalActionResp {
  id: string;
  status: string;
  message: string;
}

export interface ApprovalResp {
  id: string;
  action_id: string;
  status: string;
  exec_id: string;
  requested_by: string;
  created_at: string;
  updated_at: string;
}

export interface ApprovalDetailsResp {
  id: string;
  action_id: string;
  status: string;
  exec_id: string;
  inputs?: any;
  flow_name: string;
  flow_id: string;
  requested_by: string;
  approved_by?: string;
  created_at: string;
  updated_at: string;
}

// Execution types
export type ExecutionStatus =
  | "cancelled"
  | "pending"
  | "completed"
  | "errored"
  | "pending_approval"
  | "running";

export interface ExecutionSummary {
  id: string;
  flow_name: string;
  flow_id: string;
  status: ExecutionStatus;
  trigger_type: string;
  input?: any;
  triggered_by: string;
  current_action_id: string;
  started_at: string;
  completed_at: string;
  duration: string;
}

// Pagination types
export interface PaginateRequest {
  filter?: string;
  page?: number;
  count_per_page?: number;
}

export interface ApprovalPaginateRequest extends PaginateRequest {
  status?: "pending" | "approved" | "rejected" | "";
}

export interface PaginatedResponse<T> {
  page_count: number;
  total_count: number;
}

export interface UsersPaginateResponse
  extends PaginatedResponse<UserWithGroups> {
  users: UserWithGroups[];
}

export interface GroupsPaginateResponse
  extends PaginatedResponse<GroupWithUsers> {
  groups: GroupWithUsers[];
}

export interface FlowsPaginateResponse extends PaginatedResponse<FlowListItem> {
  flows: FlowListItem[];
}

export interface NodesPaginateResponse extends PaginatedResponse<NodeResp> {
  nodes: NodeResp[];
}

export interface CredentialsPaginateResponse
  extends PaginatedResponse<CredentialResp> {
  credentials: CredentialResp[];
}

// Flow secrets types
export interface FlowSecretReq {
  key: string;
  value: string;
  description?: string;
}

export interface FlowSecretResp {
  id: string;
  key: string;
  description?: string;
  created_at: string;
  updated_at: string;
}

export interface ApprovalsPaginateResponse
  extends PaginatedResponse<ApprovalResp> {
  approvals: ApprovalResp[];
}

export interface ExecutionsPaginateResponse
  extends PaginatedResponse<ExecutionSummary> {
  executions: ExecutionSummary[];
}

// Group access types
export interface GroupAccessReq {
  group_id: string;
}

// Additional request types from swagger
export interface CreateUserReq {
  name: string;
  username: string;
}

export interface UpdateUserReq {
  name: string;
  username: string;
  groups: string[];
}

export interface CreateGroupReq {
  name: string;
  description?: string;
}

// Executor types
export interface ExecutorConfigResponse {
  [key: string]: any;
}

// Error types
export interface FlowInputValidationError {
  field: string;
  error: string;
}

export interface ApiErrorResponse {
  error: string;
  code?: string;
  details?: Record<string, string> | FlowInputValidationError;
}

// Flow creation types
export interface FlowCreateReq {
  metadata: FlowMetaReq;
  inputs: FlowInputReq[];
  actions: FlowActionReq[];
  outputs?: Record<string, any>[];
}

export interface FlowMetaReq {
  name: string;
  description?: string;
  schedules?: string[];
}

export interface FlowInputReq {
  name: string;
  type:
    | "string"
    | "number"
    | "password"
    | "file"
    | "datetime"
    | "checkbox"
    | "select";
  label?: string;
  description?: string;
  validation?: string;
  required?: boolean;
  default?: string;
  options?: string[];
}

export interface FlowActionReq {
  name: string;
  executor: "script" | "docker";
  with: Record<string, any>;
  approval?: boolean;
  variables?: Record<string, any>[];
  artifacts?: string[];
  condition?: string;
  on?: string[];
}

export interface FlowCreateResp {
  id: string;
}

export interface FlowUpdateReq {
  schedules: string[];
  inputs: FlowInputReq[];
  actions: FlowActionReq[];
  outputs?: Record<string, any>[];
}

// Table component types
export interface TableColumn<T = any> {
  key: string;
  header: string;
  width?: string;
  render?: (value: any, row: T) => string;
  component?: any;
  sortable?: boolean;
}

export interface TableAction<T = any> {
  label: string;
  onClick: (row: T, event?: Event) => void;
  className?: string;
}

export interface TableProps<T = any> {
  columns: TableColumn<T>[];
  data: T[];
  onRowClick?: (row: T) => void;
  actions?: TableAction<T>[];
  loading?: boolean;
  emptyMessage?: string;
  emptyIcon?: string;
}
