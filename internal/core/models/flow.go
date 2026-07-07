package models

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/cvhariharan/flowctl/internal/scheduler"
	"github.com/expr-lang/expr"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/huml-lang/go-huml"
	"gopkg.in/yaml.v3"
)

type InputType string

const (
	INPUT_TYPE_STRING   InputType = "string"
	INPUT_TYPE_NUMBER   InputType = "number"
	INPUT_TYPE_PASSWORD InputType = "password"
	INPUT_TYPE_FILE     InputType = "file"
	INPUT_TYPE_DATETIME InputType = "datetime"
	INPUT_TYPE_CHECKBOX InputType = "checkbox"
	INPUT_TYPE_SELECT   InputType = "select"
)

type RemoteOptions struct {
	URL     string            `yaml:"url" huml:"url" json:"url"`
	Method  string            `yaml:"method,omitempty" huml:"method" json:"method,omitempty"`
	Headers map[string]string `yaml:"headers,omitempty" huml:"headers" json:"headers,omitempty"`
}

type Input struct {
	Name          string         `yaml:"name" huml:"name" json:"name" validate:"required,alphanum_underscore"`
	Type          InputType      `yaml:"type" huml:"type" json:"type" validate:"required,oneof=string number password file datetime checkbox select"`
	Label         string         `yaml:"label" huml:"label" json:"label"`
	Description   string         `yaml:"description" huml:"description" json:"description"`
	Validation    string         `yaml:"validation" huml:"validation" json:"validation"`
	Required      bool           `yaml:"required" huml:"required" json:"required"`
	Default       string         `yaml:"default" huml:"default" json:"default"`
	Options       []string       `yaml:"options" huml:"options" json:"options"`
	MaxFileSize   int64          `yaml:"max_file_size" huml:"max_file_size" json:"max_file_size"`
	RemoteOptions *RemoteOptions `yaml:"remote_options,omitempty" huml:"remote_options" json:"remote_options,omitempty"`
}

// type Schedule struct {
// 	Cron     string `yaml:"cron" huml:"cron" json:"cron" validate:"required,cron"`
// 	Timezone string `yaml:"timezone" huml:"timezone" json:"timezone" validate:"required,timezone"`
// }

type NotifyEvent string

const (
	NotifyEventOnSuccess   NotifyEvent = "on_success"
	NotifyEventOnFailure   NotifyEvent = "on_failure"
	NotifyEventOnWaiting   NotifyEvent = "on_waiting"
	NotifyEventOnCancelled NotifyEvent = "on_cancelled"
)

type Notify struct {
	Channel string         `yaml:"channel" huml:"channel" json:"channel" validate:"required,oneof=email webhook"`
	Config  map[string]any `yaml:"config" huml:"config" json:"config" validate:"required"`
	Events  []NotifyEvent  `yaml:"events" huml:"events" json:"events" validate:"required,dive,min=1,oneof=on_success on_failure on_waiting on_cancelled"`
}

type Action struct {
	ID             string         `yaml:"id" huml:"id" validate:"required,alphanum_underscore"`
	Name           string         `yaml:"name" huml:"name" validate:"required"`
	Executor       string         `yaml:"executor" huml:"executor"`
	With           map[string]any `yaml:"with" huml:"with" validate:"required"`
	Approval       bool           `yaml:"approval" huml:"approval"`
	Approvers      []string       `yaml:"approvers,omitempty" huml:"approvers,omitempty"`
	ApprovalGroups []string       `yaml:"approval_groups,omitempty" huml:"approval_groups,omitempty"`
	Variables      []Variable     `yaml:"variables" huml:"variables"`
	On             []string       `yaml:"on" huml:"on"`
}

func SchedulerActionToAction(a scheduler.Action) Action {
	var variables []Variable
	for _, v := range a.Variables {
		variables = append(variables, Variable(v))
	}

	var nodeNames []string
	for _, node := range a.On {
		nodeNames = append(nodeNames, node.Name)
	}

	return Action{
		ID:             a.ID,
		Name:           a.Name,
		With:           a.With,
		On:             nodeNames,
		Executor:       a.Executor,
		Approval:       a.Approval,
		Approvers:      a.Approvers,
		ApprovalGroups: a.ApprovalGroups,
		Variables:      variables,
	}
}

type Metadata struct {
	ID              string `yaml:"id" huml:"id" validate:"required,alphanum_underscore"`
	DBID            int32  `yaml:"-" huml:"-"`
	Name            string `yaml:"name" huml:"name" validate:"required,alphanum_whitespace,min=1,max=150"`
	Description     string `yaml:"description" huml:"description" validate:"max=255,no_html"`
	SrcDir          string `yaml:"-" huml:"-"`
	Namespace       string `yaml:"namespace" huml:"namespace"`
	Prefix          string `yaml:"prefix" huml:"prefix" validate:"omitempty,alphanum_underscore,max=100"`
	AllowOverlap    bool   `yaml:"allow_overlap" huml:"allow_overlap"`
	UserSchedulable bool   `yaml:"user_schedulable" huml:"user_schedulable"`
	MaxRetries      int    `yaml:"max_retries" huml:"max_retries" validate:"omitempty,min=0,max=10"`
}

type Variable map[string]any

func (v Variable) Valid() bool {
	return !(len(v) > 1)
}

func (v Variable) Name() string {
	if !v.Valid() {
		return ""
	}

	for k := range v {
		return k
	}
	return ""
}

func (v Variable) Value() string {
	if !v.Valid() {
		return ""
	}

	for _, v := range v {
		if str, ok := v.(string); ok {
			return str
		}
	}
	return ""
}

type Output map[string]any

type FlowValidationError struct {
	FieldName string
	Msg       string
	Err       error
}

func (f *FlowValidationError) Error() string {
	return fmt.Sprintf("Field: %s, %s: %v", f.FieldName, f.Msg, f.Err)
}

type Flow struct {
	Meta      Metadata   `yaml:"metadata" huml:"metadata" validate:"required"`
	Inputs    []Input    `yaml:"inputs" huml:"inputs" validate:"required,dive"`
	Actions   []Action   `yaml:"actions" huml:"actions" validate:"required,dive"`
	Outputs   []Output   `yaml:"outputs" huml:"outputs"`
	Schedules []Schedule `yaml:"schedules" huml:"schedules" validate:"omitempty,dive"`
	Notify    []Notify   `yaml:"notify" huml:"notify" json:"notify" validate:"omitempty,dive"`
}

func AlphanumericUnderscore(fl validator.FieldLevel) bool {
	regex := regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
	value := fl.Field().String()

	return regex.MatchString(value)
}

func AlphanumericSpace(fl validator.FieldLevel) bool {
	regex := regexp.MustCompile(`^[a-zA-Z0-9 _\.\-:/()]+$`)
	value := fl.Field().String()

	return regex.MatchString(value)
}

func NamespaceName(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	if value != strings.TrimSpace(value) {
		return false
	}

	regex := regexp.MustCompile(`^[a-zA-Z0-9_ -]+$`)
	return regex.MatchString(value)
}

func NoHTML(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	return !strings.Contains(value, "<") && !strings.Contains(value, ">")
}

// ValidNotificationReceiver validates notification receiver format
// Receivers must be either a valid email address or group reference "group:name"
func ValidNotificationReceiver(fl validator.FieldLevel) bool {
	receiver := fl.Field().String()

	// Check for control characters that could enable injection attacks
	if strings.ContainsAny(receiver, "\r\n\x00") {
		return false
	}

	if groupName, ok := strings.CutPrefix(receiver, "group:"); ok {
		return len(groupName) > 0 && len(groupName) <= 100
	}

	// Validate as email if not group
	if len(receiver) < 3 || len(receiver) > 254 {
		return false
	}

	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9.!#$%&'*+/=?^_` + "`" + `{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`)
	return emailRegex.MatchString(receiver)
}

func (f Flow) Validate() error {
	validate := validator.New()

	validate.RegisterValidation("alphanum_underscore", AlphanumericUnderscore)
	validate.RegisterValidation("alphanum_whitespace", AlphanumericSpace)
	validate.RegisterValidation("no_html", NoHTML)

	actionsIDs := make(map[string]int)
	for _, action := range f.Actions {
		// Check if action IDs are unique
		if _, ok := actionsIDs[action.ID]; ok {
			return fmt.Errorf("action ID %s is reused, actions IDs should be unique", action.ID)
		}
		actionsIDs[action.ID] = 1
	}

	// Validate default values for inputs
	for _, input := range f.Inputs {
		if err := validateDefaultValue(input); err != nil {
			return fmt.Errorf("validation error for input %s: %w", input.Name, err)
		}
	}

	// Reject reserved prefix values that collide with Casbin domain sentinels
	if f.Meta.Prefix == "_" {
		return fmt.Errorf("prefix %q is reserved and cannot be used", f.Meta.Prefix)
	}

	// Validate remote_options URLs
	for _, input := range f.Inputs {
		if input.RemoteOptions != nil {
			if input.RemoteOptions.URL == "" {
				return fmt.Errorf("input %s: remote_options.url is required", input.Name)
			}
			u, err := url.Parse(input.RemoteOptions.URL)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
				return fmt.Errorf("input %s: remote_options.url must be a valid http or https URL", input.Name)
			}
		}
	}

	return validate.Struct(f)
}

func (f Flow) GetActionIndexByID(id string) (int, error) {
	for i, v := range f.Actions {
		if v.ID == id {
			return i, nil
		}
	}

	return -1, fmt.Errorf("action %s not found", id)
}

func (f Flow) IsApprovalRequired() bool {
	for _, action := range f.Actions {
		if action.Approval {
			return true
		}
	}
	return false
}

// validateDefaultValue validates that a default value matches the expected input type
func validateDefaultValue(input Input) error {
	if input.Type == INPUT_TYPE_FILE && input.Default != "" {
		return fmt.Errorf("file inputs cannot have default values")
	}

	if input.Default == "" {
		return nil
	}

	switch input.Type {
	case INPUT_TYPE_CHECKBOX:
		if input.Default != "true" && input.Default != "false" {
			return fmt.Errorf("default for checkbox must be 'true' or 'false'")
		}
	case INPUT_TYPE_NUMBER:
		if _, err := strconv.ParseFloat(input.Default, 64); err != nil {
			return fmt.Errorf("default for number must be a valid number")
		}
	case INPUT_TYPE_SELECT:
		if len(input.Options) > 0 && !slices.Contains(input.Options, input.Default) {
			return fmt.Errorf("default for select must be one of the options")
		}
	}
	return nil
}

func (f Flow) ValidateInput(inputs map[string]interface{}) *FlowValidationError {
	for _, input := range f.Inputs {
		value, exists := inputs[input.Name]
		if !exists {
			if input.Required {
				return &FlowValidationError{FieldName: input.Name, Msg: "This is a required field"}
			}
			continue
		}

		if err := validateType(input.Name, value, InputType(input.Type)); err != nil {
			return &FlowValidationError{FieldName: input.Name, Msg: "Wrong input type"}
		}

		// If this is a select type, check that the value is in the list
		if input.Type == INPUT_TYPE_SELECT {
			if !slices.Contains(input.Options, value.(string)) {
				return &FlowValidationError{FieldName: input.Name, Msg: "The selected value is not part of the list"}
			}
		}

		if input.Validation == "" {
			continue
		}

		env := map[string]interface{}{
			input.Name: value,
		}

		program, err := expr.Compile(input.Validation, expr.Env(env))
		if err != nil {
			return &FlowValidationError{FieldName: input.Name, Msg: "Failed running validation", Err: err}
		}

		output, err := expr.Run(program, env)
		if err != nil {
			return &FlowValidationError{FieldName: input.Name, Msg: "Failed running validation", Err: err}
		}

		valid, ok := output.(bool)
		if !ok {
			return &FlowValidationError{FieldName: input.Name, Msg: "Failed running validation", Err: fmt.Errorf("error running validation for input %s: expected boolean response", input.Name)}
		}

		if !valid {
			return &FlowValidationError{FieldName: input.Name, Msg: fmt.Sprintf("Validation failed: %s", input.Validation)}
		}
	}

	return nil
}

func validateType(name string, val interface{}, t InputType) error {
	switch t {
	case INPUT_TYPE_STRING, INPUT_TYPE_PASSWORD, INPUT_TYPE_FILE, INPUT_TYPE_DATETIME, INPUT_TYPE_SELECT:
		_, ok := val.(string)
		if !ok {
			return fmt.Errorf("input %s must be a string", name)
		}
	case INPUT_TYPE_NUMBER:
		switch val.(type) {
		case int, int64, float64:
			// Already valid number types (for direct API calls)
		default:
			return fmt.Errorf("input %s must be a number", name)
		}
	case INPUT_TYPE_CHECKBOX:
		_, ok := val.(bool)
		if !ok {
			return fmt.Errorf("input %s must be a boolean", name)
		}
	default:
		return fmt.Errorf("unknown input type: %s", t)
	}

	return nil
}

type Execution struct {
	Input       map[string]interface{} `json:"input"`
	ExecID      string                 `json:"exec_id"`
	Version     int64                  `json:"version"`
	ErrorMsg    string                 `json:"error_msg"`
	TriggeredBy string                 `json:"triggered_by"`
}

// FlowFormat represents the file format for flows
type FlowFormat string

const (
	FlowFormatYAML FlowFormat = "yaml"
	FlowFormatHUML FlowFormat = "huml"
)

// UnmarshalFlow unmarshals flow data from either YAML or HUML format
func UnmarshalFlow(data []byte, format FlowFormat) (Flow, error) {
	var f Flow
	var err error

	switch format {
	case FlowFormatHUML:
		err = huml.Unmarshal(data, &f)
	case FlowFormatYAML:
		err = yaml.Unmarshal(data, &f)
	default:
		return Flow{}, fmt.Errorf("unsupported flow format: %s", format)
	}

	if err != nil {
		return Flow{}, fmt.Errorf("failed to unmarshal flow: %w", err)
	}

	return f, nil
}

// MarshalFlow marshals a flow to either YAML or HUML format
func MarshalFlow(f Flow, format FlowFormat) ([]byte, error) {
	var data []byte
	var err error

	switch format {
	case FlowFormatHUML:
		data, err = huml.Marshal(f)
	case FlowFormatYAML:
		data, err = yaml.Marshal(f)
	default:
		return nil, fmt.Errorf("unsupported flow format: %s", format)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to marshal flow: %w", err)
	}

	return data, nil
}

// ParseActionTargets parses the On array and separates node names from tag references.
// Tag references are prefixed with "tag:" (e.g., "tag:web").
// Returns two slices: node names and tags (without the "tag:" prefix).
func ParseActionTargets(on []string) (nodeNames []string, tags []string) {
	for _, item := range on {
		if tagName, ok := strings.CutPrefix(item, "tag:"); ok {
			tags = append(tags, tagName)
		} else {
			nodeNames = append(nodeNames, item)
		}
	}
	return nodeNames, tags
}

// ConvertToSchedulerFlow converts a Flow to scheduler.Flow
func ConvertToSchedulerFlow(ctx context.Context, f Flow, namespaceUUID uuid.UUID, getNodesByNames func(context.Context, []string, uuid.UUID) ([]Node, error), getNodesByTags func(context.Context, []string, uuid.UUID) ([]Node, error)) (scheduler.Flow, error) {
	// Convert inputs
	var inputs []scheduler.Input
	for _, inp := range f.Inputs {
		inputs = append(inputs, scheduler.Input{
			Name:        inp.Name,
			Type:        scheduler.InputType(inp.Type),
			Label:       inp.Label,
			Description: inp.Description,
			Validation:  inp.Validation,
			Required:    inp.Required,
			Default:     inp.Default,
			MaxFileSize: inp.MaxFileSize,
		})
	}

	var actions []scheduler.Action
	for _, act := range f.Actions {
		nodeNames, tags := ParseActionTargets(act.On)

		var nodes []Node
		if len(nodeNames) > 0 {
			nodesByName, err := getNodesByNames(ctx, nodeNames, namespaceUUID)
			if err != nil {
				return scheduler.Flow{}, fmt.Errorf("failed to get nodes by names for action %s: %w", act.ID, err)
			}
			nodes = append(nodes, nodesByName...)
		}

		if len(tags) > 0 {
			nodesByTags, err := getNodesByTags(ctx, tags, namespaceUUID)
			if err != nil {
				return scheduler.Flow{}, fmt.Errorf("failed to get nodes by tag for action %s: %w", act.ID, err)
			}
			// Deduplicate nodes
			seen := make(map[string]bool)
			for _, n := range nodes {
				seen[n.ID] = true
			}
			for _, n := range nodesByTags {
				if !seen[n.ID] {
					nodes = append(nodes, n)
					seen[n.ID] = true
				}
			}
		}

		// Convert nodes to scheduler format
		var schedulerNodes []scheduler.Node
		for _, node := range nodes {
			schedulerNodes = append(schedulerNodes, scheduler.Node{
				ID:             node.ID,
				Name:           node.Name,
				Hostname:       node.Hostname,
				Port:           node.Port,
				Username:       node.Username,
				OSFamily:       node.OSFamily,
				ConnectionType: node.ConnectionType,
				Tags:           node.Tags,
				Auth: scheduler.NodeAuth{
					CredentialID: node.Auth.CredentialID,
					Method:       scheduler.AuthMethod(node.Auth.Method),
					Key:          node.Auth.Key,
				},
			})
		}

		// Convert variables
		var variables []scheduler.Variable
		for _, v := range act.Variables {
			variables = append(variables, scheduler.Variable(v))
		}

		actions = append(actions, scheduler.Action{
			ID:        act.ID,
			Name:      act.Name,
			Executor:  act.Executor,
			With:      act.With,
			Approval:  act.Approval,
			Variables: variables,
			On:        schedulerNodes,
		})
	}

	// Convert outputs
	var outputs []scheduler.Output
	for _, out := range f.Outputs {
		outputs = append(outputs, scheduler.Output(out))
	}

	// Convert schedules
	var schedules []scheduler.Scheduling
	for _, sched := range f.Schedules {
		schedules = append(schedules, scheduler.Scheduling{
			Cron:     sched.Cron,
			Timezone: sched.Timezone,
		})
	}

	// Convert notify configurations
	var notify []scheduler.Notify
	for _, n := range f.Notify {
		var events []scheduler.NotifyEvent
		for _, e := range n.Events {
			events = append(events, scheduler.NotifyEvent(e))
		}
		notify = append(notify, scheduler.Notify{
			Channel: n.Channel,
			Config:  n.Config,
			Events:  events,
		})
	}

	return scheduler.Flow{
		Meta: scheduler.Metadata{
			ID:          f.Meta.ID,
			DBID:        f.Meta.DBID,
			Name:        f.Meta.Name,
			Description: f.Meta.Description,
			SrcDir:      f.Meta.SrcDir,
			Namespace:   f.Meta.Namespace,
			MaxRetries:  f.Meta.MaxRetries,
		},
		Inputs:    inputs,
		Actions:   actions,
		Outputs:   outputs,
		Schedules: schedules,
		Notify:    notify,
	}, nil
}
