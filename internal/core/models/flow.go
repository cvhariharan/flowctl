package models

import (
	"context"
	"fmt"
	"regexp"
	"slices"

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

type Input struct {
	Name        string    `yaml:"name" huml:"name" json:"name" validate:"required,alphanum_underscore"`
	Type        InputType `yaml:"type" huml:"type" json:"type" validate:"required,oneof=string number password file datetime checkbox select"`
	Label       string    `yaml:"label" huml:"label" json:"label"`
	Description string    `yaml:"description" huml:"description" json:"description"`
	Validation  string    `yaml:"validation" huml:"validation" json:"validation"`
	Required    bool      `yaml:"required" huml:"required" json:"required"`
	Default     string    `yaml:"default" huml:"default" json:"default"`
	Options     []string  `yaml:"options" huml:"options" json:"options"`
}

type Action struct {
	ID        string         `yaml:"id" huml:"id" validate:"required,alphanum_underscore"`
	Name      string         `yaml:"name" huml:"name" validate:"required"`
	Executor  string         `yaml:"executor" huml:"executor" validate:"required,oneof=script docker"`
	With      map[string]any `yaml:"with" huml:"with" validate:"required"`
	Approval  bool           `yaml:"approval" huml:"approval"`
	Variables []Variable     `yaml:"variables" huml:"variables"`
	On        []string       `yaml:"on" huml:"on"`
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
		ID:        a.ID,
		Name:      a.Name,
		With:      a.With,
		On:        nodeNames,
		Executor:  a.Executor,
		Approval:  a.Approval,
		Variables: variables,
	}
}

type Metadata struct {
	ID           string   `yaml:"id" huml:"id" validate:"required,alphanum_underscore"`
	DBID         int32    `yaml:"-" huml:"-"`
	Name         string   `yaml:"name" huml:"name" validate:"required"`
	Description  string   `yaml:"description" huml:"description"`
	Schedules    []string `yaml:"schedules" huml:"schedules" validate:"omitempty,dive,cron"`
	SrcDir       string   `yaml:"-" huml:"-"`
	Namespace    string   `yaml:"namespace" huml:"namespace"`
	AllowOverlap bool     `yaml:"allow_overlap" huml:"allow_overlap"`
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
	Meta    Metadata `yaml:"metadata" huml:"metadata" validate:"required"`
	Inputs  []Input  `yaml:"inputs" huml:"inputs" validate:"required,dive"`
	Actions []Action `yaml:"actions" huml:"actions" validate:"required,dive"`
	Outputs []Output `yaml:"outputs" huml:"outputs"`
}

func AlphanumericUnderscore(fl validator.FieldLevel) bool {
	regex := regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
	value := fl.Field().String()

	return regex.MatchString(value)
}

func AlphanumericSpace(fl validator.FieldLevel) bool {
	regex := regexp.MustCompile(`^[a-zA-Z0-9 ]+$`)
	value := fl.Field().String()

	return regex.MatchString(value)
}

func (f Flow) Validate() error {
	validate := validator.New()

	validate.RegisterValidation("alphanum_underscore", AlphanumericUnderscore)

	actionsIDs := make(map[string]int)
	for _, action := range f.Actions {
		// Check if action IDs are unique
		if _, ok := actionsIDs[action.ID]; ok {
			return fmt.Errorf("action ID %s is reused, actions IDs should be unique", action.ID)
		}
		actionsIDs[action.ID] = 1
	}

	// Check if schedules are set on a non-schedulable flow
	if len(f.Meta.Schedules) > 0 && !f.IsSchedulable() {
		return fmt.Errorf("cannot set schedules on flow: flow has inputs without default values")
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

func (f Flow) IsSchedulable() bool {
	for _, input := range f.Inputs {
		if input.Default == "" {
			return false
		}
	}
	return true
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

// convertToSchedulerFlow converts a Flow to scheduler.Flow
func ConvertToSchedulerFlow(ctx context.Context, f Flow, namespaceUUID uuid.UUID, getNodesByNames func(context.Context, []string, uuid.UUID) ([]Node, error)) (scheduler.Flow, error) {
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
		})
	}

	// Convert actions
	var actions []scheduler.Action
	for _, act := range f.Actions {
		// Get nodes for this action
		nodes, err := getNodesByNames(ctx, act.On, namespaceUUID)
		if err != nil && len(act.On) > 0 {
			return scheduler.Flow{}, fmt.Errorf("failed to get nodes for action %s: %w", act.ID, err)
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

	return scheduler.Flow{
		Meta: scheduler.Metadata{
			ID:          f.Meta.ID,
			DBID:        f.Meta.DBID,
			Name:        f.Meta.Name,
			Description: f.Meta.Description,
			Schedules:   f.Meta.Schedules,
			SrcDir:      f.Meta.SrcDir,
			Namespace:   f.Meta.Namespace,
		},
		Inputs:  inputs,
		Actions: actions,
		Outputs: outputs,
	}, nil
}
