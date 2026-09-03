package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/cvhariharan/flowctl/internal/messengers"
	"github.com/cvhariharan/flowctl/internal/repo"
	"github.com/google/uuid"
)

const PayloadTypeNotification PayloadType = "notification"

type NotificationPayload struct {
	FlowID      string         `json:"flow_id"`
	FlowName    string         `json:"flow_name"`
	ExecID      string         `json:"exec_id"`
	Status      string         `json:"status"`
	Error       string         `json:"error,omitempty"`
	Config      map[string]any `json:"config"`
	NamespaceID string         `json:"namespace_id"`
	Channel     string         `json:"channel"`
	TriggerType string         `json:"trigger_type,omitempty"`
	UserUUID    string         `json:"user_uuid,omitempty"`
}

// NotificationHandler processes notification jobs
type NotificationHandler struct {
	messengers map[string]messengers.Messenger
	store      repo.Store
	logger     *slog.Logger
}

func NewNotificationHandler(m map[string]messengers.Messenger, store repo.Store, logger *slog.Logger) *NotificationHandler {
	return &NotificationHandler{
		messengers: m,
		store:      store,
		logger:     logger,
	}
}

func (h *NotificationHandler) Type() PayloadType {
	return PayloadTypeNotification
}

func (h *NotificationHandler) Handle(ctx context.Context, job Job) error {
	var payload NotificationPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal notification payload: %w", err)
	}

	h.logger.Debug("processing notification", "flow_id", payload.FlowID, "exec_id", payload.ExecID, "status", payload.Status, "channel", payload.Channel)

	// Route to messenger by channel name
	messenger, ok := h.messengers[payload.Channel]
	if !ok {
		h.logger.Warn("no messenger configured for channel", "channel", payload.Channel)
		return nil
	}

	namespaceUUID, err := uuid.Parse(payload.NamespaceID)
	if err != nil {
		return err
	}
	namespace, err := h.store.GetNamespaceByUUID(ctx, namespaceUUID)
	if err != nil {
		return fmt.Errorf("could not get namespace name for %s: %w", payload.NamespaceID, err)
	}

	msg := messengers.Message{
		Event: messengers.EventFlowExecution,
		Data: messengers.FlowExecutionEvent{
			FlowID:      payload.FlowID,
			FlowName:    payload.FlowName,
			ExecID:      payload.ExecID,
			Status:      payload.Status,
			Error:       payload.Error,
			Namespace:   namespace.Name,
			TriggerType: payload.TriggerType,
			TriggeredBy: h.resolveUserName(ctx, payload.UserUUID),
			StartedAt:   h.executionStartedAt(ctx, payload.ExecID),
		},
		Config: payload.Config,
	}

	if err := messenger.Send(ctx, msg); err != nil {
		return fmt.Errorf("failed to send notification via %s: %w", payload.Channel, err)
	}

	h.logger.Info("notification sent", "flow_id", payload.FlowID, "exec_id", payload.ExecID, "channel", payload.Channel)

	return nil
}

func (h *NotificationHandler) resolveUserName(ctx context.Context, userUUID string) string {
	if userUUID == "" {
		return ""
	}

	parsed, err := uuid.Parse(userUUID)
	if err != nil {
		return ""
	}

	user, err := h.store.GetUserByUUID(ctx, parsed)
	if err != nil {
		h.logger.Warn("could not resolve notification user", "user_uuid", userUUID, "error", err)
		return ""
	}

	if user.Name != "" {
		return user.Name
	}
	return user.Username
}

func (h *NotificationHandler) executionStartedAt(ctx context.Context, execID string) *time.Time {
	exec, err := h.store.GetExecutionProjection(ctx, execID)
	if err != nil {
		h.logger.Warn("could not load execution for notification", "exec_id", execID, "error", err)
		return nil
	}

	if !exec.StartedAt.Valid {
		return nil
	}
	return &exec.StartedAt.Time
}
