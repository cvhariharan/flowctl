package models

type NamespaceRole string

const (
	NamespaceRoleUser     NamespaceRole = "user"
	NamespaceRoleReviewer NamespaceRole = "reviewer"
	NamespaceRoleAdmin    NamespaceRole = "admin"
)

type Resource string

const (
	ResourceFlow       Resource = "flow"
	ResourceFlowSecret Resource = "flow_secret"
	ResourceNode       Resource = "node"
	ResourceCredential Resource = "credential"
	ResourceMember     Resource = "member"
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
	ID          string
	SubjectID   string
	SubjectType string
	NamespaceID string
	Role        NamespaceRole
	CreatedAt   string
	UpdatedAt   string
	Name        string
}
