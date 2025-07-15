package handlers

import "github.com/cvhariharan/autopilot/internal/core/models"

type AuthReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type User struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	Name      string `json:"name"`
	LoginType string `json:"login_type"`
	Role      string `json:"role"`
}

type UserWithGroups struct {
	User
	Groups []Group `json:"groups"`
}

type Group struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Users       []User `json:"users"`
}

type GroupWithUsers struct {
	Group
	Users []User `json:"users"`
}

func coreUsertoUser(u models.User) User {
	return User{
		ID:        u.ID,
		Name:      u.Name,
		Username:  u.Username,
		LoginType: string(u.LoginType),
		Role:      string(u.Role),
	}
}

func coreGroupArrayCast(gu []models.Group) []Group {
	g := make([]Group, 0)
	for _, v := range gu {
		g = append(g, coreGroupToGroup(v))
	}
	return g
}

func coreUserArrayCast(gu []models.User) []User {
	u := make([]User, 0)
	for _, v := range gu {
		u = append(u, coreUsertoUser(v))
	}
	return u
}

func coreGroupToGroup(gu models.Group) Group {
	return Group{
		ID:          gu.ID,
		Name:        gu.Name,
		Description: gu.Description,
	}
}

type FlowInputValidationError struct {
	FieldName  string `json:"field"`
	ErrMessage string `json:"error"`
}

type FlowLogResp struct {
	MType   string            `json:"message_type"`
	Value   string            `json:"value"`
	Results map[string]string `json:"results,omitempty"`
}

type PaginateRequest struct {
	Filter string `query:"filter"`
	Page   int    `query:"page"`
	Count  int    `query:"count_per_page"`
}

type UsersPaginateResponse struct {
	Users      []UserWithGroups `json:"users"`
	PageCount  int64            `json:"page_count"`
	TotalCount int64            `json:"total_count"`
}

type GroupsPaginateResponse struct {
	Groups     []GroupWithUsers `json:"groups"`
	PageCount  int64            `json:"page_count"`
	TotalCount int64            `json:"total_count"`
}

type ApprovalActionReq struct {
	Action string `json:"action"`
}

type ApprovalActionResp struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Message string `json:"messages"`
}

// Node related types
type NodeAuth struct {
	Method       string `json:"method" validate:"required,oneof=ssh_key password"`
	CredentialID string `json:"credential_id" validate:"required,uuid"`
}

type NodeReq struct {
	Name     string   `json:"name" validate:"required,min=3,max=255"`
	Hostname string   `json:"hostname" validate:"required"`
	Port     int      `json:"port" validate:"required,min=1,max=65535"`
	Username string   `json:"username" validate:"required,min=1,max=255"`
	OSFamily string   `json:"os_family" validate:"required,oneof=linux windows"`
	Tags     []string `json:"tags"`
	Auth     NodeAuth `json:"auth" validate:"required"`
}

type NodeResp struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Hostname string   `json:"hostname"`
	Port     int      `json:"port"`
	Username string   `json:"username"`
	OSFamily string   `json:"os_family"`
	Tags     []string `json:"tags"`
	Auth     NodeAuth `json:"auth"`
}

type NodesPaginateResponse struct {
	Nodes      []NodeResp `json:"nodes"`
	PageCount  int64      `json:"page_count"`
	TotalCount int64      `json:"total_count"`
}

func coreNodeToNodeResp(n *models.Node) NodeResp {
	return NodeResp{
		ID:       n.ID,
		Name:     n.Name,
		Hostname: n.Hostname,
		Port:     n.Port,
		Username: n.Username,
		OSFamily: n.OSFamily,
		Tags:     n.Tags,
		Auth: NodeAuth{
			Method:       string(n.Auth.Method),
			CredentialID: n.Auth.CredentialID,
		},
	}
}

func coreNodeArrayToNodeRespArray(nodes []*models.Node) []NodeResp {
	resp := make([]NodeResp, len(nodes))
	for i, n := range nodes {
		resp[i] = coreNodeToNodeResp(n)
	}
	return resp
}

// Credential related types
type CredentialReq struct {
	Name       string `json:"name" validate:"required,min=3,max=255"`
	PrivateKey string `json:"private_key"`
	Password   string `json:"password"`
}

type CredentialResp struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	PrivateKey string `json:"private_key"`
	Password   string `json:"password"`
}

type CredentialsPaginateResponse struct {
	Credentials []CredentialResp `json:"credentials"`
	PageCount   int64            `json:"page_count"`
	TotalCount  int64            `json:"total_count"`
}

func coreCredentialToCredentialResp(c *models.Credential) CredentialResp {
	return CredentialResp{
		ID:         c.ID,
		Name:       c.Name,
		PrivateKey: c.PrivateKey,
		Password:   c.Password,
	}
}

func coreCredentialArrayToCredentialRespArray(creds []*models.Credential) []CredentialResp {
	resp := make([]CredentialResp, len(creds))
	for i, c := range creds {
		resp[i] = coreCredentialToCredentialResp(c)
	}
	return resp
}

// Namespace related types
type NamespaceReq struct {
	Name string `json:"name" validate:"required,min=1,max=150"`
}

type NamespaceResp struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type NamespacesPaginateResponse struct {
	Namespaces []NamespaceResp `json:"namespaces"`
	PageCount  int64           `json:"page_count"`
	TotalCount int64           `json:"total_count"`
}

func coreNamespaceToNamespaceResp(n models.Namespace) NamespaceResp {
	return NamespaceResp{
		ID:   n.ID,
		Name: n.Name,
	}
}

func coreNamespaceArrayToNamespaceRespArray(namespaces []models.Namespace) []NamespaceResp {
	resp := make([]NamespaceResp, len(namespaces))
	for i, n := range namespaces {
		resp[i] = coreNamespaceToNamespaceResp(n)
	}
	return resp
}
