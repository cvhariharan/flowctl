package models

import "time"

// APIKey represents an API key for token-based authentication
type APIKey struct {
	ID         string
	Name       string
	KeyPrefix  string
	UserID     string
	ExpiresAt  *time.Time
	LastUsedAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// APIKeyWithRawKey includes the raw key (only returned on creation)
type APIKeyWithRawKey struct {
	APIKey
	RawKey string
}

// IsExpired returns true if the API key has expired
func (k *APIKey) IsExpired() bool {
	if k.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*k.ExpiresAt)
}
