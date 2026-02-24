package models

import "time"

const TimeFormat = time.RFC3339

type Credential struct {
	ID            string
	Name          string
	KeyType       string
	KeyData       string
	NamespaceUUID string
	LastAccessed  string
}
