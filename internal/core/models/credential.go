package models

import (
	"time"

	"github.com/cvhariharan/flowctl/internal/repo"
)

const TimeFormat = "2006-01-02T15:04:05Z"

type Credential struct {
	ID            string
	Name          string
	KeyType       string
	KeyData       string
	NamespaceUUID string
	LastAccessed  string
}

func RepoCredentialToCredential(repoCred repo.Credential) *Credential {
	var lastAccessed time.Time
	if repoCred.LastAccessed.Valid {
		lastAccessed = repoCred.LastAccessed.Time
	}
	return &Credential{
		ID:           repoCred.Uuid.String(),
		Name:         repoCred.Name,
		KeyType:      repoCred.KeyType,
		KeyData:      repoCred.KeyData,
		LastAccessed: lastAccessed.Format(TimeFormat),
	}
}

func RepoCredentialByUUIDToCredential(repoCred repo.GetCredentialByUUIDRow) *Credential {
	var lastAccessed time.Time
	if repoCred.LastAccessed.Valid {
		lastAccessed = repoCred.LastAccessed.Time
	}
	return &Credential{
		ID:           repoCred.Uuid.String(),
		Name:         repoCred.Name,
		KeyType:      repoCred.KeyType,
		KeyData:      repoCred.KeyData,
		LastAccessed: lastAccessed.Format(TimeFormat),
	}
}

func RepoCredentialListToCredential(repoCreds []repo.ListCredentialsRow) []*Credential {
	results := make([]*Credential, 0, len(repoCreds))
	for _, cred := range repoCreds {
		var lastAccessed time.Time
		if cred.LastAccessed.Valid {
			lastAccessed = cred.LastAccessed.Time
		}
		results = append(results, &Credential{
			ID:           cred.Uuid.String(),
			Name:         cred.Name,
			KeyType:      cred.KeyType,
			KeyData:      cred.KeyData,
			LastAccessed: lastAccessed.Format(TimeFormat),
		})
	}
	return results
}
