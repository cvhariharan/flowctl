package streamlogger

import (
	"bytes"
	"encoding/gob"
)

type MessageType string

const (
	LogMessageType    MessageType = "log"
	ErrMessageType    MessageType = "error"
	ResultMessageType MessageType = "result"
	StateMessageType  MessageType = "state"
)

type StreamMessage struct {
	ActionID string      `json:"action_id"`
	MType    MessageType `json:"message_type"`
	Val      []byte      `json:"value"`
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

	return nil
}
