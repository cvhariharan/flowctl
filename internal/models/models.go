package models

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"time"
)

type LogMessage struct {
	Message    string
	Results    map[string]string
	Checkpoint bool
	ID         string
	Err        string
}

type ExecutionCheckpoint struct {
	ActionID string
	Err      string
	Results  map[string]string
}

// MarshalBinary implements the encoding.BinaryMarshaler interface.
// It serializes the ExecutionCheckpoint into a binary format.
func (ec ExecutionCheckpoint) MarshalBinary() ([]byte, error) {
	var buf bytes.Buffer

	// Register the types with gob to ensure proper encoding
	gob.Register(map[string]string{})

	// Create a new encoder
	enc := gob.NewEncoder(&buf)

	// Encode the struct fields
	if err := enc.Encode(ec.ActionID); err != nil {
		return nil, fmt.Errorf("failed to encode ActionID: %w", err)
	}
	if err := enc.Encode(ec.Err); err != nil {
		return nil, fmt.Errorf("failed to encode Err: %w", err)
	}
	if err := enc.Encode(ec.Results); err != nil {
		return nil, fmt.Errorf("failed to encode Results: %w", err)
	}

	return buf.Bytes(), nil
}

// UnmarshalBinary implements the encoding.BinaryUnmarshaler interface.
// It deserializes the binary data back into an ExecutionCheckpoint.
func (ec *ExecutionCheckpoint) UnmarshalBinary(data []byte) error {
	buf := bytes.NewBuffer(data)

	// Register the types with gob to ensure proper decoding
	gob.Register(map[string]string{})

	// Create a new decoder
	dec := gob.NewDecoder(buf)

	// Decode the struct fields
	if err := dec.Decode(&ec.ActionID); err != nil {
		return fmt.Errorf("failed to decode ActionID: %w", err)
	}
	if err := dec.Decode(&ec.Err); err != nil {
		return fmt.Errorf("failed to decode Err: %w", err)
	}
	if err := dec.Decode(&ec.Results); err != nil {
		return fmt.Errorf("failed to decode Results: %w", err)
	}

	return nil
}

type UserInfo struct {
	ID       int32    `json:"-"`
	UUID     string   `json:"-"`
	Subject  string   `json:"sub"`
	Email    string   `json:"email"`
	Password string   `json:"-"`
	Name     string   `json:"name"`
	Groups   []string `json:"groups"`
}

type ExecutionStatus string

const (
	ExecutionStatusPending   ExecutionStatus = "pending"
	ExecutionStatusCompleted ExecutionStatus = "completed"
	ExecutionStatusErrored   ExecutionStatus = "errored"
)

type ExecutionSummary struct {
	ExecID      string
	Flow        Flow
	Status      ExecutionStatus
	TriggeredBy string
	CreatedAt   time.Time
	CompletedAt time.Time
}

func (e ExecutionSummary) Duration() string {
	duration := e.CompletedAt.Sub(e.CreatedAt)

	// Handle durations less than a minute
	if duration < time.Minute {
		if duration < time.Second {
			return fmt.Sprintf("%d milliseconds", duration.Milliseconds())
		}
		return fmt.Sprintf("%d seconds", int(duration.Seconds()))
	}

	// Handle durations less than an hour
	if duration < time.Hour {
		minutes := int(duration.Minutes())
		seconds := int(duration.Seconds()) % 60
		if seconds == 0 {
			return fmt.Sprintf("%d minutes", minutes)
		}
		return fmt.Sprintf("%d minutes %d seconds", minutes, seconds)
	}

	// Handle durations of an hour or more
	hours := int(duration.Hours())
	minutes := int(duration.Minutes()) % 60
	if minutes == 0 {
		return fmt.Sprintf("%d hours", hours)
	}
	return fmt.Sprintf("%d hours %d minutes", hours, minutes)
}
