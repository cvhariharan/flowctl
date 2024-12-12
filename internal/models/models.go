package models

type Input struct {
	Name        string `yaml:"name" json:"name"`
	Type        string `yaml:"type" json:"type"`
	Label       string `yaml:"label" json:"label"`
	Description string `yaml:"description" json:"description"`
	Validation  string `yaml:"validation" json:"validation"`
	Required    bool   `yaml:"required" json:"required"`
	Default     string `yaml:"default" json:"default"`
}

type Action struct {
	ID         string     `yaml:"id"`
	Name       string     `yaml:"name"`
	Image      string     `yaml:"image"`
	Variables  []Variable `yaml:"variables"`
	Script     []string   `yaml:"script"`
	Entrypoint []string   `yaml:"entrypoint"`
	Artifacts  []string   `yaml:"artifacts"`
	Condition  string     `yaml:"condition"`
}

type Metadata struct {
	ID          string `yaml:"id"`
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

type Variable map[string]any

type Output map[string]any

type Flow struct {
	Meta    Metadata `yaml:"metadata"`
	Inputs  []Input  `yaml:"inputs"`
	Actions []Action `yaml:"actions"`
	Outputs []Output `yaml:"outputs"`
}
