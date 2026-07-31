package models

type NamespaceRole string

const (
	NamespaceRoleUser     NamespaceRole = "user"
	NamespaceRoleOperator NamespaceRole = "operator"
	NamespaceRoleReviewer NamespaceRole = "reviewer"
	NamespaceRoleAdmin    NamespaceRole = "admin"
)

type Resource string

const (
	ResourceFlow            Resource = "flow"
	ResourceFlowSecret      Resource = "flow_secret"
	ResourceNamespaceSecret Resource = "namespace_secret"
	ResourceNode            Resource = "node"
	ResourceCredential      Resource = "credential"
	ResourceMember          Resource = "member"
	ResourceExecution       Resource = "execution"
	ResourceApproval        Resource = "approval"
	ResourceNamespace       Resource = "namespace"
)

type RBACAction string

const (
	RBACActionView       RBACAction = "view"
	RBACActionViewConfig RBACAction = "view_config"
	RBACActionExecute    RBACAction = "execute"
	RBACActionApprove    RBACAction = "approve"
	RBACActionUpdate     RBACAction = "update"
	RBACActionDelete     RBACAction = "delete"
	RBACActionCreate     RBACAction = "create"
	RBACActionExport     RBACAction = "export"
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

type FlowPrefix struct {
	ID          string
	Name        string
	Description string
}

type PrefixAccess struct {
	Prefix    string
	CreatedAt string
}

// ValidResource checks if the given resource is a known RBAC resource.
func ValidResource(r Resource) bool {
	switch r {
	case ResourceFlow, ResourceFlowSecret, ResourceNamespaceSecret, ResourceNode,
		ResourceCredential, ResourceMember, ResourceExecution, ResourceApproval, ResourceNamespace:
		return true
	default:
		return false
	}
}

// ValidRBACAction checks if the given action is a known RBAC action.
func ValidRBACAction(a RBACAction) bool {
	switch a {
	case RBACActionView, RBACActionViewConfig, RBACActionExecute, RBACActionApprove,
		RBACActionUpdate, RBACActionDelete, RBACActionCreate, RBACActionExport:
		return true
	default:
		return false
	}
}
