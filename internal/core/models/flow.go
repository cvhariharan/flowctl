package models

import (
	"fmt"
	"regexp"
	"slices"

	"github.com/cvhariharan/flowctl/internal/scheduler"
	"github.com/cvhariharan/flowctl/internal/tasks"
	"github.com/expr-lang/expr"
	"github.com/go-playground/validator/v10"
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
	Name        string    `yaml:"name" json:"name" validate:"required,alphanum_underscore"`
	Type        InputType `yaml:"type" json:"type" validate:"required,oneof=string number password file datetime checkbox select"`
	Label       string    `yaml:"label" json:"label"`
	Description string    `yaml:"description" json:"description"`
	Validation  string    `yaml:"validation" json:"validation"`
	Required    bool      `yaml:"required" json:"required"`
	Default     string    `yaml:"default" json:"default"`
	Options     []string  `yaml:"options" json:"options"`
}

type Action struct {
	ID        string         `yaml:"id" validate:"required,alphanum_underscore"`
	Name      string         `yaml:"name" validate:"required"`
	Executor  string         `yaml:"executor" validate:"required,oneof=script docker"`
	With      map[string]any `yaml:"with" validate:"required"`
	Approval  bool           `yaml:"approval"`
	Variables []Variable     `yaml:"variables"`
	Artifacts []string       `yaml:"artifacts"`
	Condition string         `yaml:"condition"`
	On        []string       `yaml:"on"`
}

func TaskActionToAction(a tasks.Action) Action {
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
		Artifacts: a.Artifacts,
		Condition: a.Condition,
	}
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
		Artifacts: a.Artifacts,
		Condition: a.Condition,
	}
}

type Metadata struct {
	ID          string `yaml:"id" validate:"required,alphanum_underscore"`
	DBID        int32  `yaml:"-"`
	Name        string `yaml:"name" validate:"required"`
	Description string `yaml:"description"`
	Schedule    string `yaml:"schedule" validate:"omitempty,cron"`
	SrcDir      string `yaml:"-"`
	Namespace   string `yaml:"namespace"`
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
	Meta    Metadata `yaml:"metadata" validate:"required"`
	Inputs  []Input  `yaml:"inputs" validate:"required,dive"`
	Actions []Action `yaml:"actions" validate:"required,dive"`
	Outputs []Output `yaml:"outputs"`
}

func ToTaskFlowModel(f Flow, nodeLookupFunc func(nodeNames []string) ([]Node, error)) (tasks.Flow, error) {
	var ti []tasks.Input
	for _, v := range f.Inputs {
		ti = append(ti, tasks.Input{
			Name:        v.Name,
			Type:        tasks.InputType(v.Type),
			Label:       v.Label,
			Description: v.Description,
			Default:     v.Default,
			Required:    v.Required,
			Validation:  v.Validation,
		})
	}

	var ta []tasks.Action
	for _, v := range f.Actions {

		var tvs []tasks.Variable
		for _, val := range v.Variables {
			tvs = append(tvs, tasks.Variable(val))
		}

		nodes, err := nodeLookupFunc(v.On)
		if err != nil {
			return tasks.Flow{}, fmt.Errorf("error looking up nodes for action %s: %w", v.ID, err)
		}

		ta = append(ta, tasks.Action{
			ID:        v.ID,
			Name:      v.Name,
			With:      v.With,
			On:        NodesToTaskNodesModel(nodes),
			Executor:  v.Executor,
			Approval:  v.Approval,
			Variables: tvs,
			Artifacts: v.Artifacts,
			Condition: v.Condition,
		})
	}

	var to []tasks.Output
	for _, v := range f.Outputs {
		to = append(to, tasks.Output(v))
	}

	tf := tasks.Flow{
		Meta:    tasks.Metadata(f.Meta),
		Inputs:  ti,
		Actions: ta,
		Outputs: to,
	}

	return tf, nil
}

func NodesToTaskNodesModel(nodes []Node) []tasks.Node {
	var tn []tasks.Node
	for _, n := range nodes {
		tn = append(tn, tasks.Node{
			ID:       n.ID,
			Name:     n.Name,
			Hostname: n.Hostname,
			Port:     n.Port,
			Username: n.Username,
			OSFamily: n.OSFamily,
			Tags:     n.Tags,
			ConnectionType: n.ConnectionType,
			Auth: tasks.NodeAuth{
				CredentialID: n.Auth.CredentialID,
				Method:       tasks.AuthMethod(n.Auth.Method),
				Key:          n.Auth.Key,
			},
		})
	}
	return tn
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

	// Check if schedule is set on a non-schedulable flow
	if f.Meta.Schedule != "" && !f.IsSchedulable() {
		return fmt.Errorf("cannot set schedule on flow: flow has inputs without default values")
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
		if input.Type == INPUT_TYPE_SELECT  {
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
