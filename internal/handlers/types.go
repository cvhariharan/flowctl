package handlers

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/cvhariharan/flowctl/internal/core/models"
	"github.com/gosimple/slug"
)

const (
	TimeFormat = time.RFC3339
)

// GenerateSlug creates a slug from the provided string
// The slug uses only alphabets, numbers, and underscores
func GenerateSlug(input string) string {
	return strings.ReplaceAll(slug.Make(input), "-", "_")
}

type SSOProvider struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type AuthReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type FlowTriggerResp struct {
	ExecID      string  `json:"exec_id"`
	ScheduledAt *string `json:"scheduled_at,omitempty"`
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
	ActionID  string            `json:"action_id"`
	MType     string            `json:"message_type"`
	NodeID    string            `json:"node_id"`
	Value     string            `json:"value"`
	Timestamp string            `json:"timestamp"`
	Results   map[string]string `json:"results,omitempty"`
}

type PaginateRequest struct {
	Filter string `query:"filter"`
	Page   int    `query:"page"`
	Count  int    `query:"count_per_page"`
}

type NodePaginateRequest struct {
	Filter string   `query:"filter"`
	Tags   []string `query:"tags"`
	Page   int      `query:"page"`
	Count  int      `query:"count_per_page"`
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
	ApprovalID string `param:"approvalID" validate:"required,uuid4"`
	Action     string `json:"action" validate:"required,oneof=approve reject"`
}

type ApprovalGetReq struct {
	ApprovalID string `param:"approvalID" validate:"required,uuid4"`
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
	FlowName    string `json:"flow_name"`
	Status      string `json:"status"`
	ExecID      string `json:"exec_id"`
	RequestedBy string `json:"requested_by"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type ApprovalDetailsResp struct {
	ID             string          `json:"id"`
	ActionID       string          `json:"action_id"`
	Status         string          `json:"status"`
	ExecID         string          `json:"exec_id"`
	Inputs         json.RawMessage `json:"inputs,omitempty"`
	FlowName       string          `json:"flow_name"`
	FlowID         string          `json:"flow_id"`
	DecidedBy      string          `json:"approved_by"`
	RequestedBy    string          `json:"requested_by"`
	Approvers      []string        `json:"approvers,omitempty"`
	ApprovalGroups []string        `json:"approval_groups,omitempty"`
	CreatedAt      string          `json:"created_at"`
	UpdatedAt      string          `json:"updated_at"`
}

type ApprovalsPaginateResponse struct {
	Approvals  []ApprovalResp `json:"approvals"`
	PageCount  int64          `json:"page_count"`
	TotalCount int64          `json:"total_count"`
}

// Node related types
type NodeAuth struct {
	Method       string `json:"method" validate:"required,oneof=private_key password"`
	CredentialID string `json:"credential_id" validate:"required,uuid4"`
}

type NodeReq struct {
	Name           string   `json:"name" validate:"required,min=1,max=50,alphanum_underscore"`
	Hostname       string   `json:"hostname" validate:"required,hostname|ip"`
	Port           int      `json:"port" validate:"required,min=1,max=65535"`
	Username       string   `json:"username" validate:"required,min=2,max=50"`
	ConnectionType string   `json:"connection_type" validate:"required,oneof=ssh qssh"`
	Tags           []string `json:"tags" validate:"omitempty,dive,alphanum_underscore"`
	Auth           NodeAuth `json:"auth" validate:"required"`
	// OSFamily       string   `json:"os_family" validate:"required,oneof=linux windows"`
}

type NodeResp struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Hostname       string   `json:"hostname"`
	Port           int      `json:"port"`
	Username       string   `json:"username"`
	OSFamily       string   `json:"os_family"`
	ConnectionType string   `json:"connection_type"`
	Tags           []string `json:"tags"`
	Auth           NodeAuth `json:"auth"`
}

type NodesPaginateResponse struct {
	Nodes      []NodeResp `json:"nodes"`
	PageCount  int64      `json:"page_count"`
	TotalCount int64      `json:"total_count"`
}

type NodeStatsResp struct {
	TotalHosts int64 `json:"total_hosts"`
	SSHHosts   int64 `json:"ssh_hosts"`
	QSSHHosts  int64 `json:"qssh_hosts"`
}

func coreNodeToNodeResp(n models.Node) NodeResp {
	return NodeResp{
		ID:             n.ID,
		Name:           n.Name,
		Hostname:       n.Hostname,
		Port:           n.Port,
		Username:       n.Username,
		OSFamily:       n.OSFamily,
		ConnectionType: n.ConnectionType,
		Tags:           n.Tags,
		Auth: NodeAuth{
			Method:       string(n.Auth.Method),
			CredentialID: n.Auth.CredentialID,
		},
	}
}

func coreNodeArrayToNodeRespArray(nodes []models.Node) []NodeResp {
	resp := make([]NodeResp, len(nodes))
	for i, n := range nodes {
		resp[i] = coreNodeToNodeResp(n)
	}
	return resp
}

// Credential related types
type CredentialReq struct {
	Name    string `json:"name" validate:"required,min=2,max=255,alphanum_whitespace"`
	KeyType string `json:"key_type" validate:"required,oneof=private_key password"`
	KeyData string `json:"key_data" validate:"required"`
}

type CredentialGetReq struct {
	CredID string `param:"credID" validate:"required,uuid4"`
}

type CredentialUpdateReq struct {
	CredentialGetReq
	CredentialReq
}

type CredentialResp struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	KeyType      string `json:"key_type"`
	LastAccessed string `json:"last_accessed"`
}

type CredentialsPaginateResponse struct {
	Credentials []CredentialResp `json:"credentials"`
	PageCount   int64            `json:"page_count"`
	TotalCount  int64            `json:"total_count"`
}

func coreCredentialToCredentialResp(c models.Credential) CredentialResp {
	return CredentialResp{
		ID:           c.ID,
		Name:         c.Name,
		KeyType:      c.KeyType,
		LastAccessed: c.LastAccessed,
	}
}

func coreCredentialArrayToCredentialRespArray(creds []models.Credential) []CredentialResp {
	resp := make([]CredentialResp, len(creds))
	for i, c := range creds {
		resp[i] = coreCredentialToCredentialResp(c)
	}
	return resp
}

// Namespace related types
type NamespaceReq struct {
	Name string `json:"name" validate:"required,min=1,max=150,namespace_name"`
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

// Schedule represents a cron schedule with timezone
type Schedule struct {
	Cron     string `json:"cron"`
	Timezone string `json:"timezone"`
}

// Notify represents notification configuration for flow events
type Notify struct {
	Channel string         `json:"channel" validate:"required,oneof=email webhook"`
	Config  map[string]any `json:"config" validate:"required"`
	Events  []string       `json:"events" validate:"required,dive,min=1,oneof=on_success on_failure on_waiting on_cancelled"`
}

func convertNotifyToNotifyReq(notify []models.Notify) []Notify {
	resp := make([]Notify, len(notify))
	for i, n := range notify {
		events := make([]string, len(n.Events))
		for j, e := range n.Events {
			events[j] = string(e)
		}
		resp[i] = Notify{
			Channel: n.Channel,
			Config:  n.Config,
			Events:  events,
		}
	}
	return resp
}

func convertNotifyReqToNotify(notify []Notify) []models.Notify {
	resp := make([]models.Notify, len(notify))
	for i, n := range notify {
		events := make([]models.NotifyEvent, len(n.Events))
		for j, e := range n.Events {
			events[j] = models.NotifyEvent(e)
		}
		resp[i] = models.Notify{
			Channel: n.Channel,
			Config:  n.Config,
			Events:  events,
		}
	}
	return resp
}

// Flow list response type
type FlowListItem struct {
	ID          string     `json:"id"`
	Slug        string     `json:"slug"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Prefix      string     `json:"prefix"`
	Schedules   []Schedule `json:"schedules"`
	StepCount   int        `json:"step_count"`
}

type FlowInput struct {
	Name        string   `json:"name"`
	Label       string   `json:"label"`
	Description string   `json:"description"`
	Required    bool     `json:"required"`
	Type        string   `json:"type"`
	Options     []string `json:"options"`
	Default     string   `json:"default,omitempty"`
	MaxFileSize int64    `json:"max_file_size,omitempty"`
}

type FlowInputsResp struct {
	Inputs           []FlowInput `json:"inputs"`
	OptionsRequestID string      `json:"options_request_id,omitempty"`
}

func coreFlowInputToInput(input models.Input) FlowInput {
	return FlowInput{
		Name:        input.Name,
		Description: input.Description,
		Label:       input.Label,
		Required:    input.Required,
		Type:        string(input.Type),
		Options:     input.Options,
		Default:     input.Default,
		MaxFileSize: input.MaxFileSize,
	}
}

func coreFlowInputsToInputs(inputs []models.Input) []FlowInput {
	flowInputs := make([]FlowInput, 0)
	for _, i := range inputs {
		flowInputs = append(flowInputs, coreFlowInputToInput(i))
	}
	return flowInputs
}

type FlowMeta struct {
	ID              string     `json:"id"`
	Name            string     `json:"name" validate:"required,min=1,max=150,alphanum_whitespace"`
	Description     string     `json:"description" validate:"max=255,no_html"`
	Prefix          string     `json:"prefix" validate:"omitempty,alphanum_underscore,max=100"`
	Schedules       []Schedule `json:"schedules"`
	Namespace       string     `json:"namespace"`
	AllowOverlap    bool       `json:"allow_overlap"`
	UserSchedulable bool       `json:"user_schedulable"`
	MaxRetries      int        `json:"max_retries" validate:"omitempty,min=0,max=10"`
}

func coreSchedulesToSchedules(schedules []models.Schedule) []Schedule {
	resp := make([]Schedule, len(schedules))
	for i, s := range schedules {
		resp[i] = Schedule{
			Cron:     s.Cron,
			Timezone: s.Timezone,
		}
	}
	return resp
}

func coreFlowMetatoFlowMeta(m models.Metadata, schedules []models.Schedule) FlowMeta {
	return FlowMeta{
		ID:              m.ID,
		Name:            m.Name,
		Description:     m.Description,
		Prefix:          m.Prefix,
		Schedules:       coreSchedulesToSchedules(schedules),
		Namespace:       m.Namespace,
		AllowOverlap:    m.AllowOverlap,
		UserSchedulable: m.UserSchedulable,
		MaxRetries:      m.MaxRetries,
	}
}

type FlowAction struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Executor       string   `json:"executor"`
	Approval       bool     `json:"approval"`
	On             []string `json:"on"`
	Approvers      []string `json:"approvers,omitempty"`
	ApprovalGroups []string `json:"approval_groups,omitempty"`
}

func coreFlowActiontoFlowAction(a models.Action) FlowAction {
	return FlowAction{
		ID:             a.ID,
		Name:           a.Name,
		Executor:       a.Executor,
		Approval:       a.Approval,
		On:             a.On,
		Approvers:      a.Approvers,
		ApprovalGroups: a.ApprovalGroups,
	}
}

func coreFlowActionstoFlowActions(a []models.Action) []FlowAction {
	f := make([]FlowAction, 0)
	for _, v := range a {
		f = append(f, coreFlowActiontoFlowAction(v))
	}
	return f
}

type FlowMetaResp struct {
	Metadata            FlowMeta             `json:"meta"`
	Actions             []FlowAction         `json:"actions"`
	ScheduledExecutions []ScheduledExecution `json:"scheduled_executions"`
}

type ScheduledExecution struct {
	ExecID      string `json:"exec_id"`
	ScheduledAt string `json:"scheduled_at"`
}

type FlowListResponse struct {
	Flows []FlowListItem `json:"flows"`
}

type FlowsPaginateResponse struct {
	Flows      []FlowListItem `json:"flows"`
	PageCount  int64          `json:"page_count"`
	TotalCount int64          `json:"total_count"`
}

type ExecutionsPaginateResponse struct {
	Executions []ExecutionSummary `json:"executions"`
	PageCount  int64              `json:"page_count"`
	TotalCount int64              `json:"total_count"`
}

type UserReq struct {
	Name     string   `json:"name" validate:"required,min=2,max=50,alphanum_whitespace"`
	Username string   `json:"username" validate:"required,email"`
	Groups   []string `json:"groups"`
}

type ChangePasswordReq struct {
	OldPassword string `json:"old_password" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=8"`
}

type GroupReq struct {
	Name        string `json:"name" validate:"required,alphanum_underscore,min=1,max=50"`
	Description string `json:"description" validate:"max=255"`
}

func coreFlowToFlow(flow models.Flow) FlowListItem {
	return FlowListItem{
		ID:          flow.Meta.ID,
		Slug:        flow.Meta.ID,
		Name:        flow.Meta.Name,
		Description: flow.Meta.Description,
		Prefix:      flow.Meta.Prefix,
		Schedules:   coreSchedulesToSchedules(flow.Schedules),
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
	ID       string   `json:"id"`
	Username string   `json:"username"`
	Name     string   `json:"name"`
	Role     string   `json:"role"`
	Groups   []string `json:"groups"`
}

type AppInfoResponse struct {
	Version         string `json:"version"`
	Commit          string `json:"commit"`
	BuildDate       string `json:"build_date"`
	DefaultTimezone string `json:"default_timezone"`
}

func coreUserInfoToUserProfile(u models.UserInfo) UserProfileResponse {
	return UserProfileResponse{
		ID:       u.ID,
		Username: u.Username,
		Name:     u.Name,
		Role:     string(u.Role),
		Groups:   u.Groups,
	}
}

// Namespace member related types
type NamespaceMemberReq struct {
	SubjectID   string `json:"subject_id" validate:"required,uuid4"`
	SubjectType string `json:"subject_type" validate:"required,oneof=user group"`
	Role        string `json:"role" validate:"required,oneof=user operator reviewer admin"`
}

type UpdateNamespaceMemberReq struct {
	Role string `json:"role" validate:"required,oneof=user operator reviewer admin"`
}

type NamespaceMemberResp struct {
	ID          string `json:"id"`
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
		ID:          m.ID,
		SubjectID:   m.SubjectID,
		SubjectName: getSubjectName(m),
		SubjectType: m.SubjectType,
		Role:        string(m.Role),
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}

func getSubjectName(m models.NamespaceMember) string {
	return m.Name
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
	ExecutionStatusCancelled ExecutionStatus = "cancelled"
	ExecutionStatusPending   ExecutionStatus = "pending"
	ExecutionStatusCompleted ExecutionStatus = "completed"
	ExecutionStatusErrored   ExecutionStatus = "errored"
)

type ExecutionSummary struct {
	ID              string          `json:"id"`
	FlowName        string          `json:"flow_name"`
	FlowID          string          `json:"flow_id"`
	Status          ExecutionStatus `json:"status"`
	TriggerType     string          `json:"trigger_type"`
	Input           json.RawMessage `json:"input,omitempty"`
	TriggeredBy     string          `json:"triggered_by"`
	CurrentActionID string          `json:"current_action_id"`
	CreatedAt       string          `json:"created_at"`
	StartedAt       string          `json:"started_at"`
	CompletedAt     string          `json:"completed_at"`
	ScheduledAt     string          `json:"scheduled_at,omitempty"`
	ActionRetries   map[string]int  `json:"action_retries,omitempty"`
}

func coreExecutionSummaryToExecutionSummary(e models.ExecutionSummary) ExecutionSummary {
	completedAt := ""
	if !e.CompletedAt.IsZero() {
		completedAt = e.CompletedAt.Format(TimeFormat)
	}

	scheduledAt := ""
	if !e.ScheduledAt.IsZero() {
		scheduledAt = e.ScheduledAt.Format(TimeFormat)
	}

	startedAt := ""
	if !e.StartedAt.IsZero() {
		startedAt = e.StartedAt.Format(TimeFormat)
	}

	return ExecutionSummary{
		ID:              e.ExecID,
		FlowName:        e.FlowName,
		FlowID:          e.FlowID,
		Status:          ExecutionStatus(e.Status),
		Input:           e.Input,
		TriggerType:     e.TriggerType,
		TriggeredBy:     e.TriggeredByName,
		CurrentActionID: e.CurrentActionID,
		CreatedAt:       e.CreatedAt.Format(TimeFormat),
		StartedAt:       startedAt,
		CompletedAt:     completedAt,
		ScheduledAt:     scheduledAt,
		ActionRetries:   e.ActionRetries,
	}
}

type FlowCreateReq struct {
	Meta          FlowMeta        `json:"metadata" validate:"required"`
	Inputs        []FlowInputReq  `json:"inputs" validate:"required,dive"`
	Actions       []FlowActionReq `json:"actions" validate:"required,dive"`
	Notifications []Notify        `json:"notify" validate:"omitempty,dive"`
}

type RemoteOptionsReq struct {
	URL     string            `json:"url" validate:"required,url"`
	Method  string            `json:"method,omitempty" validate:"omitempty,oneof=GET POST"`
	Headers map[string]string `json:"headers,omitempty"`
}

type FlowInputReq struct {
	Name          string            `json:"name" validate:"required,alphanum_underscore,min=1,max=150"`
	Type          string            `json:"type" validate:"required,oneof=string number password file datetime checkbox select"`
	Label         string            `json:"label" validate:"omitempty,max=255"`
	Description   string            `json:"description" validate:"max=255"`
	Validation    string            `json:"validation"`
	Required      bool              `json:"required"`
	Default       string            `json:"default"`
	Options       []string          `json:"options"`
	MaxFileSize   int64             `json:"max_file_size"`
	RemoteOptions *RemoteOptionsReq `json:"remote_options,omitempty" validate:"omitempty"`
}

type FlowActionReq struct {
	Name           string           `json:"name" validate:"required,alphanum_whitespace,min=1,max=150"`
	Executor       string           `json:"executor"`
	With           map[string]any   `json:"with" validate:"required"`
	Approval       bool             `json:"approval"`
	Approvers      []string         `json:"approvers,omitempty"`
	ApprovalGroups []string         `json:"approval_groups,omitempty"`
	Variables      []map[string]any `json:"variables"`
	Condition      string           `json:"condition"`
	On             []string         `json:"on"`
}

type FlowCreateResp struct {
	ID string `json:"id"`
}

type FlowGetReq struct {
	FlowID string `param:"flowID" validate:"required"`
}

type LogStreamingReq struct {
	LogID string `param:"logID" validate:"required,uuid4"`
}

type ExecutionGetReq struct {
	ExecID string `param:"execID" validate:"required,uuid4"`
}

type FlowUpdateReq struct {
	Prefix          string          `json:"prefix" validate:"omitempty,max=100,alphanum_underscore"`
	Schedules       []Schedule      `json:"schedules" validate:"omitempty,dive"`
	Notify          []Notify        `json:"notify" validate:"omitempty,dive"`
	AllowOverlap    bool            `json:"allow_overlap"`
	UserSchedulable bool            `json:"user_schedulable"`
	MaxRetries      int             `json:"max_retries" validate:"omitempty,min=0,max=10"`
	Description     string          `json:"description" validate:"max=255,no_html"`
	Inputs          []FlowInputReq  `json:"inputs" validate:"required,dive"`
	Actions         []FlowActionReq `json:"actions" validate:"required,dive"`
}

// Helper functions to convert request types to models
func convertFlowInputsReqToInputs(inputsReq []FlowInputReq) []models.Input {
	inputs := make([]models.Input, len(inputsReq))
	for i, input := range inputsReq {
		var remoteOpts *models.RemoteOptions
		if input.RemoteOptions != nil {
			remoteOpts = &models.RemoteOptions{
				URL:     input.RemoteOptions.URL,
				Method:  input.RemoteOptions.Method,
				Headers: input.RemoteOptions.Headers,
			}
		}
		inputs[i] = models.Input{
			Name:          input.Name,
			Type:          models.InputType(input.Type),
			Label:         input.Label,
			Description:   input.Description,
			Validation:    input.Validation,
			Required:      input.Required,
			Default:       input.Default,
			Options:       input.Options,
			MaxFileSize:   input.MaxFileSize,
			RemoteOptions: remoteOpts,
		}
	}
	return inputs
}

func convertFlowActionsReqToActions(actionsReq []FlowActionReq) []models.Action {
	actions := make([]models.Action, len(actionsReq))
	for i, action := range actionsReq {
		// Convert variables
		variables := make([]models.Variable, len(action.Variables))
		for j, v := range action.Variables {
			variables[j] = models.Variable(v)
		}

		actions[i] = models.Action{
			ID:             GenerateSlug(action.Name),
			Name:           action.Name,
			Executor:       action.Executor,
			With:           action.With,
			Approval:       action.Approval,
			Approvers:      action.Approvers,
			ApprovalGroups: action.ApprovalGroups,
			Variables:      variables,
			On:             action.On,
		}
	}
	return actions
}

// Helper functions to convert models to request types
func convertFlowInputsToInputsReq(inputs []models.Input) []FlowInputReq {
	inputsReq := make([]FlowInputReq, len(inputs))
	for i, input := range inputs {
		var remoteOpts *RemoteOptionsReq
		if input.RemoteOptions != nil {
			remoteOpts = &RemoteOptionsReq{
				URL:     input.RemoteOptions.URL,
				Method:  input.RemoteOptions.Method,
				Headers: input.RemoteOptions.Headers,
			}
		}
		inputsReq[i] = FlowInputReq{
			Name:          input.Name,
			Type:          string(input.Type),
			Label:         input.Label,
			Description:   input.Description,
			Validation:    input.Validation,
			Required:      input.Required,
			Default:       input.Default,
			Options:       input.Options,
			MaxFileSize:   input.MaxFileSize,
			RemoteOptions: remoteOpts,
		}
	}
	return inputsReq
}

func convertFlowActionsToActionsReq(actions []models.Action) []FlowActionReq {
	actionsReq := make([]FlowActionReq, len(actions))
	for i, action := range actions {
		// Convert variables
		variables := make([]map[string]any, len(action.Variables))
		for j, v := range action.Variables {
			variables[j] = map[string]any(v)
		}

		actionsReq[i] = FlowActionReq{
			Name:           action.Name,
			Executor:       action.Executor,
			With:           action.With,
			Approval:       action.Approval,
			Approvers:      action.Approvers,
			ApprovalGroups: action.ApprovalGroups,
			Variables:      variables,
			On:             action.On,
		}
	}
	return actionsReq
}

type FlowSecretReq struct {
	FlowID      string `param:"flowID" validate:"required"`
	Key         string `json:"key" validate:"required,min=1,max=150,alphanum_underscore"`
	Value       string `json:"value" validate:"required"`
	Description string `json:"description" validate:"max=255"`
}

type FlowSecretResp struct {
	ID          string `json:"id"`
	Key         string `json:"key"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type FlowSecretGetReq struct {
	SecretID string `param:"secretID" validate:"required"`
}

type FlowSecretUpdateReq struct {
	FlowSecretGetReq
	Value       string `json:"value" validate:"required"`
	Description string `json:"description" validate:"max=255"`
}

type FlowSecretsListReq struct {
	FlowID string `param:"flowID" validate:"required"`
}

func coreFlowSecretToFlowSecretResp(secret models.FlowSecret) FlowSecretResp {
	return FlowSecretResp{
		ID:          secret.ID,
		Key:         secret.Key,
		Description: secret.Description,
		CreatedAt:   secret.CreatedAt,
		UpdatedAt:   secret.UpdatedAt,
	}
}

type NamespaceSecretReq struct {
	Key         string `json:"key" validate:"required,min=1,max=150,alphanum_underscore"`
	Value       string `json:"value" validate:"required"`
	Description string `json:"description" validate:"max=255"`
}

type NamespaceSecretResp struct {
	ID          string `json:"id"`
	Key         string `json:"key"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type NamespaceSecretGetReq struct {
	SecretID string `param:"secretID" validate:"required"`
}

type NamespaceSecretUpdateReq struct {
	NamespaceSecretGetReq
	Value       string `json:"value" validate:"required"`
	Description string `json:"description" validate:"max=255"`
}

func coreNamespaceSecretToNamespaceSecretResp(secret models.NamespaceSecret) NamespaceSecretResp {
	return NamespaceSecretResp{
		ID:          secret.ID,
		Key:         secret.Key,
		Description: secret.Description,
		CreatedAt:   secret.CreatedAt,
		UpdatedAt:   secret.UpdatedAt,
	}
}

type FlowCancellationResp struct {
	Message string `json:"message"`
	ExecID  string `json:"execID"`
}

type ScheduleCreateReq struct {
	FlowID   string                 `param:"flowID" validate:"required"`
	Cron     string                 `json:"cron" validate:"required,cron"`
	Timezone string                 `json:"timezone" validate:"required,timezone"`
	Inputs   map[string]interface{} `json:"inputs" validate:"required"`
}

type ScheduleUpdateReq struct {
	FlowID     string                 `param:"flowID" validate:"required"`
	ScheduleID string                 `param:"schedule_id" validate:"required,uuid4"`
	Cron       string                 `json:"cron" validate:"required,cron"`
	Timezone   string                 `json:"timezone" validate:"required,timezone"`
	Inputs     map[string]interface{} `json:"inputs" validate:"required"`
	IsActive   bool                   `json:"is_active"`
}

type ScheduleGetReq struct {
	FlowID     string `param:"flowID" validate:"required"`
	ScheduleID string `param:"schedule_id" validate:"required,uuid4"`
}

type ScheduleListReq struct {
	FlowID string `param:"flowID" validate:"required"`
	Page   int    `query:"page"`
	Count  int    `query:"count_per_page"`
}

type ScheduleUpdateResp struct {
	ScheduleID string `json:"schedule_id"`
}

type ScheduleResp struct {
	UUID          string                 `json:"uuid"`
	FlowSlug      string                 `json:"flow_slug"`
	FlowName      string                 `json:"flow_name"`
	Cron          string                 `json:"cron"`
	Timezone      string                 `json:"timezone"`
	Inputs        map[string]interface{} `json:"inputs"`
	CreatedBy     string                 `json:"created_by"`
	IsActive      bool                   `json:"is_active"`
	IsUserCreated bool                   `json:"is_user_created"`
	CreatedAt     string                 `json:"created_at"`
	UpdatedAt     string                 `json:"updated_at"`
}

type SchedulesPaginateResponse struct {
	Schedules  []ScheduleResp `json:"schedules"`
	PageCount  int64          `json:"page_count"`
	TotalCount int64          `json:"total_count"`
}

func coreScheduleToScheduleResp(s models.Schedule) ScheduleResp {
	return ScheduleResp{
		UUID:          s.UUID,
		FlowSlug:      s.FlowSlug,
		FlowName:      s.FlowName,
		Cron:          s.Cron,
		Timezone:      s.Timezone,
		Inputs:        s.Inputs,
		CreatedBy:     s.CreatedByID,
		IsActive:      s.IsActive,
		IsUserCreated: s.IsUserCreated,
		CreatedAt:     s.CreatedAt.Format(TimeFormat),
		UpdatedAt:     s.UpdatedAt.Format(TimeFormat),
	}
}

func coreSchedulesToScheduleResps(schedules []models.Schedule) []ScheduleResp {
	resp := make([]ScheduleResp, len(schedules))
	for i, s := range schedules {
		resp[i] = coreScheduleToScheduleResp(s)
	}
	return resp
}

// Flow group types
type FlowGroupResp struct {
	ID          string `json:"id"`
	Prefix      string `json:"prefix"`
	Description string `json:"description"`
	FlowCount   int64  `json:"flow_count"`
}

type FlowGroupsResponse struct {
	Groups []FlowGroupResp `json:"groups"`
}

type FlowGroupReq struct {
	Name        string `json:"name" validate:"required,min=1,max=100,alphanum_underscore"`
	Description string `json:"description" validate:"max=500,no_html"`
}

type FlowGroupDetailResp struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type GroupAccessReq struct {
	Prefix string `json:"prefix" validate:"required,min=1,max=100,alphanum_underscore"`
}

type GroupAccessResp struct {
	Prefix    string `json:"prefix"`
	CreatedAt string `json:"created_at"`
}

type GroupAccessListResp struct {
	Groups []GroupAccessResp `json:"groups"`
}

type ExecutorInfo struct {
	Name         string   `json:"name"`
	Capabilities []string `json:"capabilities"`
}

type ExecutorsListResponse struct {
	Executors []ExecutorInfo `json:"executors"`
}

type KVEntry struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type APITokenCreateReq struct {
	Label string `json:"label" validate:"required,max=150"`
}

type APITokenCreateResp struct {
	models.APIToken
	Token string `json:"token"`
}
