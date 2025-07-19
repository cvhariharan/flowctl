package handlers

import (
	"github.com/cvhariharan/autopilot/internal/core/models"
)

const (
	TimeFormat = "2006-01-02T15:04:05Z"
)

type AuthReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type FlowTriggerResp struct {
	ExecID string `json:"exec_id"`
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
	ActionID string			   `json:"action_id"`
	MType    string            `json:"message_type"`
	Value    string            `json:"value"`
	Results  map[string]string `json:"results,omitempty"`
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

type ApprovalPaginateRequest struct {
	Status string `query:"status" validate:"oneof='' pending approved rejected"`
	Filter string `query:"filter"`
	Page   int    `query:"page"`
	Count  int    `query:"count_per_page"`
}

type ApprovalResp struct {
	ID          string `json:"id"`
	ActionID    string `json:"action_id"`
	Status      string `json:"status"`
	ExecID      string `json:"exec_id"`
	RequestedBy string `json:"requested_by"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type ApprovalsPaginateResponse struct {
	Approvals  []ApprovalResp `json:"approvals"`
	PageCount  int64          `json:"page_count"`
	TotalCount int64          `json:"total_count"`
}

// Node related types
type NodeAuth struct {
	Method       string `json:"method" validate:"required,oneof=private_key password"`
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
	Name    string `json:"name" validate:"required,min=3,max=255"`
	KeyType string `json:"key_type" validate:"required,oneof=private_key password"`
	KeyData string `json:"key_data" validate:"required"`
}

type CredentialResp struct {
	ID      	 string `json:"id"`
	Name    	 string `json:"name"`
	KeyType 	 string `json:"key_type"`
	LastAccessed string `json:"last_accessed"`
}

type CredentialsPaginateResponse struct {
	Credentials []CredentialResp `json:"credentials"`
	PageCount   int64            `json:"page_count"`
	TotalCount  int64            `json:"total_count"`
}

func coreCredentialToCredentialResp(c *models.Credential) CredentialResp {
	return CredentialResp{
		ID:      	  c.ID,
		Name:    	  c.Name,
		KeyType:  	  c.KeyType,
		LastAccessed: c.LastAccessed,
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

// Flow list response type
type FlowListItem struct {
	ID          string `json:"id"`
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	StepCount   int    `json:"step_count"`
}

type FlowListResponse struct {
	Flows []FlowListItem `json:"flows"`
}

type FlowsPaginateResponse struct {
	Flows      []FlowListItem `json:"flows"`
	PageCount  int64          `json:"page_count"`
	TotalCount int64          `json:"total_count"`
}

func coreFlowToFlow(flow models.Flow) FlowListItem {
	return FlowListItem{
		ID:          flow.Meta.ID,
		Slug:        flow.Meta.ID,
		Name:        flow.Meta.Name,
		Description: flow.Meta.Description,
		StepCount:   len(flow.Actions),
	}
}

func coreFlowsToFlows(flows []models.Flow) FlowListResponse {
	flowItems := make([]FlowListItem, len(flows))
	for i, flow := range flows {
		flowItems[i] = coreFlowToFlow(flow)
	}
	return FlowListResponse{Flows: flowItems}
}

type UserProfileResponse struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
	Role     string `json:"role"`
}

func coreUserInfoToUserProfile(u models.UserInfo) UserProfileResponse {
	return UserProfileResponse{
		ID:       u.ID,
		Username: u.Username,
		Name:     u.Name,
		Role:     string(u.Role),
	}
}

// Namespace member related types
type NamespaceMemberReq struct {
	SubjectID   string `json:"subject_id" validate:"required,uuid"`
	SubjectType string `json:"subject_type" validate:"required,oneof=user group"`
	Role        string `json:"role" validate:"required,oneof=user operator admin"`
}

type NamespaceMemberResp struct {
	ID 			string `json:"id"`
	SubjectID   string `json:"subject_id"`
	SubjectName string `json:"subject_name"`
	SubjectType string `json:"subject_type"`
	Role        string `json:"role"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type NamespaceMembersResponse struct {
	Members []NamespaceMemberResp `json:"members"`
}

func coreNamespaceMemberToResp(m models.NamespaceMember) NamespaceMemberResp {
	return NamespaceMemberResp{
		ID: 		 m.ID,
		SubjectID:   m.SubjectID,
		SubjectName: getSubjectName(m),
		SubjectType: m.SubjectType,
		Role:        string(m.Role),
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}

func getSubjectName(m models.NamespaceMember) string {
	if m.UserName != nil {
		return *m.UserName
	}
	if m.GroupName != nil {
		return *m.GroupName
	}
	return ""
}

func coreNamespaceMembersToResp(members []models.NamespaceMember) NamespaceMembersResponse {
	resp := make([]NamespaceMemberResp, len(members))
	for i, m := range members {
		resp[i] = coreNamespaceMemberToResp(m)
	}
	return NamespaceMembersResponse{Members: resp}
}

type ExecutionStatus string
const (
	ExecutionStatusPending   ExecutionStatus = "pending"
	ExecutionStatusCompleted ExecutionStatus = "completed"
	ExecutionStatusErrored   ExecutionStatus = "errored"
)

type ExecutionSummary struct {
	ID 	   		string 		    `json:"id"`
	Status 		ExecutionStatus `json:"status"`
	TriggeredBy string     		`json:"triggered_by"`
	CreatedAt   string 	   		`json:"started_at"`
	CompletedAt string 	   		`json:"completed_at"`
	Duration    string 			`json:"duration"`
}

func coreExecutionSummaryToExecutionSummary(e models.ExecutionSummary) ExecutionSummary {
	return ExecutionSummary{
		ID:          e.ExecID,
		Status:      ExecutionStatus(e.Status),
		TriggeredBy: e.TriggeredBy,
		CreatedAt:   e.CreatedAt.Format(TimeFormat),
		CompletedAt: e.CompletedAt.Format(TimeFormat),
		Duration:    e.Duration(),
	}
}
