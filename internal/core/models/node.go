package models

type AuthMethod string

const (
	AuthMethodPrivateKey AuthMethod = "private_key"
	AuthMethodPassword AuthMethod = "password"
)

type Node struct {
	ID            string
	Name          string
	Hostname      string
	Port          int
	Username      string
	OSFamily      string
	Tags          []string
	Auth          NodeAuth
	NamespaceUUID string
}

type NodeAuth struct {
	CredentialID string
	Method       AuthMethod
	Key          string
}
