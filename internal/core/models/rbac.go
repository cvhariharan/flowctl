package models

type NamespaceRole string

const (
	NamespaceRoleUser     NamespaceRole = "user"
	NamespaceRoleOperator NamespaceRole = "operator"
	NamespaceRoleAdmin    NamespaceRole = "admin"
)

type Resource string

const (
	ResourceFlow       Resource = "flow"
	ResourceNode       Resource = "node"
	ResourceCredential Resource = "credential"
	ResourceExecution  Resource = "execution"
	ResourceApproval   Resource = "approval"
	ResourceNamespace  Resource = "namespace"
)

type RBACAction string

const (
	RBACActionView    RBACAction = "view"
	RBACActionExecute RBACAction = "execute"
	RBACActionApprove RBACAction = "approve"
	RBACActionUpdate  RBACAction = "update"
	RBACActionDelete  RBACAction = "delete"
	RBACActionCreate  RBACAction = "create"
)

type NamespaceWithRole struct {
	Namespace Namespace     `json:"namespace"`
	Role      NamespaceRole `json:"role"`
}

type NamespaceMember struct {
	ID          int           `json:"id"`
	SubjectID   int           `json:"subject_id"`
	SubjectType string        `json:"subject_type"`
	NamespaceID string        `json:"namespace_id"`
	Role        NamespaceRole `json:"role"`
	CreatedAt   string        `json:"created_at"`
	UpdatedAt   string        `json:"updated_at"`
	UserName    *string       `json:"user_name,omitempty"`
	GroupName   *string       `json:"group_name,omitempty"`
}