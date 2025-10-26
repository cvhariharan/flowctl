package models

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
)

type ApprovalType string

const (
	ApprovalStatusPending  ApprovalType = "pending"
	ApprovalStatusApproved ApprovalType = "approved"
	ApprovalStatusRejected ApprovalType = "rejected"
)

type ApprovalRequest struct {
	UUID        string
	ActionID    string
	Status      ApprovalType
	ExecID      string
	RequestedBy string
}

func (a ApprovalRequest) MarshalBinary() ([]byte, error) {
	var buf bytes.Buffer

	if err := gob.NewEncoder(&buf).Encode(a.UUID); err != nil {
		return nil, fmt.Errorf("failed to encode UUID: %w", err)
	}
	if err := gob.NewEncoder(&buf).Encode(a.ActionID); err != nil {
		return nil, fmt.Errorf("failed to encode ActionID: %w", err)
	}
	if err := gob.NewEncoder(&buf).Encode(a.Status); err != nil {
		return nil, fmt.Errorf("failed to encode Status: %w", err)
	}
	if err := gob.NewEncoder(&buf).Encode(a.ExecID); err != nil {
		return nil, fmt.Errorf("failed to encode ExecID: %w", err)
	}
	if err := gob.NewEncoder(&buf).Encode(a.RequestedBy); err != nil {
		return nil, fmt.Errorf("failed to encode RequestedBy: %w", err)
	}

	return buf.Bytes(), nil
}

func (a *ApprovalRequest) UnmarshalBinary(data []byte) error {
	buf := bytes.NewBuffer(data)

	if err := gob.NewDecoder(buf).Decode(&a.UUID); err != nil {
		return fmt.Errorf("failed to decode UUID: %w", err)
	}
	if err := gob.NewDecoder(buf).Decode(&a.ActionID); err != nil {
		return fmt.Errorf("failed to decode ActionID: %w", err)
	}
	if err := gob.NewDecoder(buf).Decode(&a.Status); err != nil {
		return fmt.Errorf("failed to decode Status: %w", err)
	}
	if err := gob.NewDecoder(buf).Decode(&a.ExecID); err != nil {
		return fmt.Errorf("failed to decode ExecID: %w", err)
	}
	if err := gob.NewDecoder(buf).Decode(&a.RequestedBy); err != nil {
		return fmt.Errorf("failed to decode RequestedBy: %w", err)
	}

	return nil
}

type ApprovalDetails struct {
	ApprovalRequest
	DecidedBy string
	Inputs    json.RawMessage
	FlowName  string
	FlowID    string
	CreatedAt string
	UpdatedAt string
}
