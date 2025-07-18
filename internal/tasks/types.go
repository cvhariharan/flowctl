package tasks

type InputType string

const (
	INPUT_TYPE_STRING       InputType = "string"
	INPUT_TYPE_INT          InputType = "int"
	INPUT_TYPE_FLOAT        InputType = "float"
	INPUT_TYPE_BOOL         InputType = "bool"
	INPUT_TYPE_SLICE_STRING InputType = "slice_string"
	INPUT_TYPE_SLICE_INT    InputType = "slice_int"
	INPUT_TYPE_SLICE_UINT   InputType = "slice_uint"
	INPUT_TYPE_SLICE_FLOAT  InputType = "slice_float"
)

type AuthMethod string

const (
	AuthMethodPrivateKey AuthMethod = "private_key"
	AuthMethodPassword AuthMethod = "password"
)

type Node struct {
	ID       string
	Name     string
	Hostname string
	Port     int
	Username string
	OSFamily string
	Tags     []string
	Auth     NodeAuth
}

type NodeAuth struct {
	CredentialID string
	Method       AuthMethod
	Key          string
}

type Input struct {
	Name        string    `yaml:"name" json:"name" validate:"required,alphanum_underscore"`
	Type        InputType `yaml:"type" json:"type" validate:"required,oneof=string int float bool slice_string slice_int slice_uint slice_float"`
	Label       string    `yaml:"label" json:"label"`
	Description string    `yaml:"description" json:"description"`
	Validation  string    `yaml:"validation" json:"validation"`
	Required    bool      `yaml:"required" json:"required"`
	Default     string    `yaml:"default" json:"default"`
}

type Action struct {
	ID        string         `yaml:"id" validate:"required,alphanum_underscore"`
	Name      string         `yaml:"name" validate:"required"`
	Executor  string         `yaml:"executor" validate:"required,oneof=script docker"`
	With      map[string]any `yaml:"with" validate:"required"`
	Approval  ApprovalList   `yaml:"approval"`
	Variables []Variable     `yaml:"variables"`
	Artifacts []string       `yaml:"artifacts"`
	Condition string         `yaml:"condition"`
	On        []Node         `yaml:"on"`
}

type Metadata struct {
	ID          string `yaml:"id" validate:"required,alphanum_underscore"`
	DBID        int32  `yaml:"-"`
	Name        string `yaml:"name" validate:"required"`
	Description string `yaml:"description"`
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

type ApprovalList []string
type Output map[string]any

type Flow struct {
	Meta    Metadata `yaml:"metadata" validate:"required"`
	Inputs  []Input  `yaml:"inputs" validate:"required"`
	Actions []Action `yaml:"actions" validate:"required"`
	Outputs []Output `yaml:"outputs"`
}
