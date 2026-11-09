package models

import "github.com/go-playground/validator/v10"

type AuthMethod string

const (
	AuthMethodPrivateKey AuthMethod = "private_key"
	AuthMethodPassword   AuthMethod = "password"
)

type Node struct {
	ID             string
	Name           string `validate:"required,min=1,max=50,alphanum_underscore"`
	Hostname       string `validate:"required,hostname|ip"`
	Port           int    `validate:"required,min=1,max=65535"`
	Username       string `validate:"required,min=2,max=50"`
	OSFamily       string
	ConnectionType string   `validate:"required,oneof=ssh qssh"`
	Tags           []string `validate:"omitempty,dive,alphanum_underscore"`
	Auth           NodeAuth `validate:"required"`
	NamespaceUUID  string
}

type NodeAuth struct {
	CredentialID string     `validate:"required,uuid4"`
	Method       AuthMethod `validate:"required,oneof=private_key password"`
	Key          string
}

type NodeStats struct {
	TotalHosts int64 `json:"total_hosts"`
	SSHHosts   int64 `json:"ssh_hosts"`
	QSSHHosts  int64 `json:"qssh_hosts"`
}

// Validate checks the rules shared by node creation, updates, and CSV imports.
func (n Node) Validate() error {
	validate := validator.New()
	validate.RegisterValidation("alphanum_underscore", AlphanumericUnderscore)
	return validate.Struct(n)
}
