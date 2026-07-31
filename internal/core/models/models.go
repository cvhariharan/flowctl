package models

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
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

type MessageType string

const (
	LogMessageType       MessageType = "log"
	ErrMessageType       MessageType = "error"
	ResultMessageType    MessageType = "result"
	ApprovalMessageType  MessageType = "approval"
	CancelledMessageType MessageType = "cancelled"
)

type StreamMessage struct {
	ActionID  string      `json:"action_id"`
	MType     MessageType `json:"message_type"`
	NodeID    string      `json:"node_id"`
	Val       string      `json:"value"`
	Timestamp string      `json:"timestamp"`
}

func (s StreamMessage) MarshalBinary() ([]byte, error) {
	var buf bytes.Buffer

	if err := gob.NewEncoder(&buf).Encode(s.ActionID); err != nil {
		return nil, err
	}
	if err := gob.NewEncoder(&buf).Encode(s.MType); err != nil {
		return nil, err
	}
	if err := gob.NewEncoder(&buf).Encode(s.Val); err != nil {
		return nil, err
	}
	if err := gob.NewEncoder(&buf).Encode(s.Timestamp); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (s *StreamMessage) UnmarshalBinary(data []byte) error {
	buf := bytes.NewBuffer(data)

	if err := gob.NewDecoder(buf).Decode(&s.ActionID); err != nil {
		return err
	}
	if err := gob.NewDecoder(buf).Decode(&s.MType); err != nil {
		return err
	}
	if err := gob.NewDecoder(buf).Decode(&s.Val); err != nil {
		return err
	}
	if err := gob.NewDecoder(buf).Decode(&s.Timestamp); err != nil {
		return err
	}

	return nil
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

type ExecutionStatus string

const (
	ExecutionStatusCancelled       ExecutionStatus = "cancelled"
	ExecutionStatusPending         ExecutionStatus = "pending"
	ExecutionStatusPendingApproval ExecutionStatus = "pending_approval"
	ExecutionStatusCompleted       ExecutionStatus = "completed"
	ExecutionStatusErrored         ExecutionStatus = "errored"
)

// ExecutionContext is the state of an execution, stored as JSON in execution_log.context.
type ExecutionContext struct {
	Inputs  map[string]any `json:"inputs"`
	Outputs map[string]any `json:"outputs"`
}

// NewExecutionContext replaces nil maps with empty ones so the stored JSON holds no nulls.
func NewExecutionContext(inputs, outputs map[string]any) ExecutionContext {
	if inputs == nil {
		inputs = make(map[string]any)
	}
	if outputs == nil {
		outputs = make(map[string]any)
	}
	return ExecutionContext{Inputs: inputs, Outputs: outputs}
}

func UnmarshalExecutionContext(b []byte) (ExecutionContext, error) {
	if len(b) == 0 {
		return NewExecutionContext(nil, nil), nil
	}

	var execCtx ExecutionContext
	if err := json.Unmarshal(b, &execCtx); err != nil {
		return ExecutionContext{}, fmt.Errorf("could not unmarshal execution context: %w", err)
	}

	return NewExecutionContext(execCtx.Inputs, execCtx.Outputs), nil
}

type ExecutionSummary struct {
	ExecID          string
	FlowName        string
	FlowID          string
	Status          ExecutionStatus
	Inputs          map[string]any
	TriggerType     string
	TriggeredByName string
	TriggeredByID   string
	CurrentActionID string
	CreatedAt       time.Time
	StartedAt       time.Time
	CompletedAt     time.Time
	ScheduledAt     time.Time
	ActionRetries   map[string]int
}

type ScheduledExecution struct {
	ExecID      string
	ScheduledAt time.Time
}
