package scheduler

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/cvhariharan/flowctl/internal/repo"
	"github.com/cvhariharan/flowctl/internal/webhooks"
	"github.com/google/uuid"
	"gocloud.dev/secrets"
)

const PayloadTypeWebhookDelivery PayloadType = "webhook_delivery"

type WebhookDeliveryPayload struct {
	DeliveryID       string `json:"delivery_id"`
	TemplateOverride string `json:"template_override,omitempty"`
}

type WebhookDeliveryHandler struct {
	store       repo.Store
	keeper      *secrets.Keeper
	taskQueuer  TaskScheduler
	logger      *slog.Logger
	rootURL     string
	sender      *webhooks.Sender
	maxAttempts int32
}

func NewWebhookDeliveryHandler(store repo.Store, keeper *secrets.Keeper, taskQueuer TaskScheduler, logger *slog.Logger, rootURL string) *WebhookDeliveryHandler {
	return &WebhookDeliveryHandler{
		store:       store,
		keeper:      keeper,
		taskQueuer:  taskQueuer,
		logger:      logger,
		rootURL:     rootURL,
		sender:      webhooks.NewSender(logger.WithGroup("webhook_sender")),
		maxAttempts: 3,
	}
}

func (h *WebhookDeliveryHandler) Type() PayloadType {
	return PayloadTypeWebhookDelivery
}

func (h *WebhookDeliveryHandler) Handle(ctx context.Context, job Job) error {
	var payload WebhookDeliveryPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal webhook payload: %w", err)
	}

	deliveryUUID, err := uuid.Parse(payload.DeliveryID)
	if err != nil {
		return fmt.Errorf("invalid delivery UUID: %w", err)
	}

	delivery, err := h.store.GetWebhookDeliveryByUUID(ctx, deliveryUUID)
	if err != nil {
		return fmt.Errorf("failed to load webhook delivery: %w", err)
	}

	webhookRow, err := h.store.GetNamespaceWebhookByID(ctx, delivery.WebhookID)
	if err != nil {
		return fmt.Errorf("failed to load webhook config: %w", err)
	}

	if !webhookRow.IsActive {
		h.logger.Warn("webhook is inactive", "delivery_id", deliveryUUID.String())
		return h.recordAttemptAndUpdate(ctx, delivery, 0, errors.New("webhook is inactive"), time.Duration(0), false, false, payload.TemplateOverride)
	}

	urlValue, err := h.decryptWebhookValue(ctx, webhookRow.EncryptedUrl)
	if err != nil {
		return h.recordAttemptAndUpdate(ctx, delivery, 0, err, time.Duration(0), false, false, payload.TemplateOverride)
	}

	headers, err := h.decryptWebhookHeaders(ctx, webhookRow.EncryptedHeaders)
	if err != nil {
		return h.recordAttemptAndUpdate(ctx, delivery, 0, err, time.Duration(0), false, false, payload.TemplateOverride)
	}

	templateBody := webhookRow.TemplateBody
	if payload.TemplateOverride != "" {
		templateBody = payload.TemplateOverride
	}

	templateData, err := h.buildTemplateData(ctx, delivery, webhookRow)
	if err != nil {
		return h.recordAttemptAndUpdate(ctx, delivery, 0, err, time.Duration(0), false, false, payload.TemplateOverride)
	}

	body, err := webhooks.RenderTemplate(templateBody, templateData)
	if err != nil {
		return h.recordAttemptAndUpdate(ctx, delivery, 0, err, time.Duration(0), false, false, payload.TemplateOverride)
	}

	resp, err := h.sender.Send(ctx, webhooks.Request{
		URL:         urlValue,
		Body:        body,
		ContentType: webhookRow.ContentType,
		Headers:     headers,
		Event:       delivery.Event,
		DeliveryID:  delivery.Uuid.String(),
	})

	statusCode := 0
	if resp.StatusCode > 0 {
		statusCode = resp.StatusCode
	}

	success := err == nil && resp.StatusCode >= 200 && resp.StatusCode < 300
	retryable := err != nil || resp.StatusCode == 429 || resp.StatusCode >= 500

	if success {
		return h.recordAttemptAndUpdate(ctx, delivery, statusCode, nil, resp.Duration, true, false, payload.TemplateOverride)
	}

	if !retryable {
		if err == nil {
			err = fmt.Errorf("unexpected status code %d", resp.StatusCode)
		}
		return h.recordAttemptAndUpdate(ctx, delivery, statusCode, err, resp.Duration, false, false, payload.TemplateOverride)
	}

	if err == nil && resp.StatusCode >= 500 {
		err = fmt.Errorf("received status code %d", resp.StatusCode)
	}
	if err == nil && resp.StatusCode == 429 {
		err = errors.New("received status code 429")
	}

	return h.recordAttemptAndUpdate(ctx, delivery, statusCode, err, resp.Duration, false, true, payload.TemplateOverride)
}

func (h *WebhookDeliveryHandler) buildTemplateData(ctx context.Context, delivery repo.WebhookDelivery, webhookRow repo.GetNamespaceWebhookByIDRow) (map[string]any, error) {
	namespace, err := h.store.GetNamespaceByUUID(ctx, webhookRow.NamespaceUuid)
	if err != nil {
		return nil, err
	}

	flowSlug := delivery.FlowID
	flowName := delivery.FlowID
	executionStatus := delivery.Event
	triggerType := ""
	triggeredBy := ""
	errorMsg := ""
	startedAt := ""
	completedAt := ""

	execRow, err := h.store.GetExecutionByExecIDWithNamespace(ctx, repo.GetExecutionByExecIDWithNamespaceParams{
		ExecID: delivery.ExecutionID,
		Uuid:   webhookRow.NamespaceUuid,
	})
	if err == nil {
		flowSlug = execRow.FlowSlug
		flowName = execRow.FlowName
		executionStatus = string(execRow.Status)
		triggerType = string(execRow.TriggerType)
		triggeredBy = execRow.TriggeredByName
		if execRow.Error.Valid {
			errorMsg = execRow.Error.String
		}
		if execRow.StartedAt.Valid {
			startedAt = execRow.StartedAt.Time.Format(webhooks.TimeFormat)
		}
		if execRow.CompletedAt.Valid {
			completedAt = execRow.CompletedAt.Time.Format(webhooks.TimeFormat)
		}
	} else {
		h.logger.Warn("failed to load execution details for webhook", "exec_id", delivery.ExecutionID, "error", err)
	}

	return webhooks.BuildTemplateData(webhooks.TemplateContext{
		FlowID:               flowSlug,
		FlowName:             flowName,
		FlowSlug:             flowSlug,
		FlowURL:              webhooks.BuildFlowURL(h.rootURL, namespace.Name, flowSlug),
		ExecutionID:          delivery.ExecutionID,
		ExecutionStatus:      executionStatus,
		ExecutionTriggerType: triggerType,
		ExecutionTriggeredBy: triggeredBy,
		ExecutionStartedAt:   startedAt,
		ExecutionCompletedAt: completedAt,
		ExecutionError:       errorMsg,
		NamespaceID:          webhookRow.NamespaceUuid.String(),
		NamespaceName:        namespace.Name,
		RootURL:              h.rootURL,
	}), nil
}

func (h *WebhookDeliveryHandler) recordAttemptAndUpdate(ctx context.Context, delivery repo.WebhookDelivery, statusCode int, sendErr error, duration time.Duration, delivered bool, retryable bool, templateOverride string) error {
	attemptNumber := delivery.AttemptCount + 1

	errorMessage := ""
	if sendErr != nil {
		errorMessage = sendErr.Error()
	}

	durationMs := sql.NullInt32{}
	if duration > 0 {
		durationMs = sql.NullInt32{Int32: int32(duration.Milliseconds()), Valid: true}
	}

	statusCodeNull := sql.NullInt32{}
	if statusCode > 0 {
		statusCodeNull = sql.NullInt32{Int32: int32(statusCode), Valid: true}
	}

	errorMessageNull := sql.NullString{}
	if errorMessage != "" {
		errorMessageNull = sql.NullString{String: errorMessage, Valid: true}
	}

	_, _ = h.store.AddWebhookDeliveryAttempt(ctx, repo.AddWebhookDeliveryAttemptParams{
		DeliveryID:    delivery.Uuid,
		AttemptNumber: attemptNumber,
		StatusCode:    statusCodeNull,
		ErrorMessage:  errorMessageNull,
		DurationMs:    durationMs,
	})

	status := "failed"
	var nextAttemptAt sql.NullTime
	var deliveredAt sql.NullTime

	if delivered {
		status = "delivered"
		deliveredAt = sql.NullTime{Time: time.Now().UTC(), Valid: true}
	} else if retryable && attemptNumber < h.maxAttempts {
		status = "pending"
		delay := retryDelay(attemptNumber)
		nextAttemptAt = sql.NullTime{Time: time.Now().UTC().Add(delay), Valid: true}
		if h.taskQueuer != nil {
			jobID := fmt.Sprintf("webhook-%s-%d", delivery.Uuid.String(), attemptNumber+1)
			_, err := h.taskQueuer.QueueScheduledTask(ctx, PayloadTypeWebhookDelivery, jobID, WebhookDeliveryPayload{
				DeliveryID:       delivery.Uuid.String(),
				TemplateOverride: templateOverride,
			}, nextAttemptAt.Time)
			if err != nil {
				h.logger.Error("failed to schedule webhook retry", "delivery_id", delivery.Uuid.String(), "error", err)
			}
		}
	}

	_, err := h.store.UpdateWebhookDelivery(ctx, repo.UpdateWebhookDeliveryParams{
		Uuid:             delivery.Uuid,
		Status:           status,
		AttemptCount:     attemptNumber,
		NextAttemptAt:    nextAttemptAt,
		LastStatusCode:   statusCodeNull,
		LastErrorMessage: errorMessageNull,
		DeliveredAt:      deliveredAt,
	})
	return err
}

func retryDelay(attempt int32) time.Duration {
	switch attempt {
	case 1:
		return 5 * time.Second
	case 2:
		return 30 * time.Second
	default:
		return 2 * time.Minute
	}
}

func (h *WebhookDeliveryHandler) decryptWebhookValue(ctx context.Context, encryptedValue string) (string, error) {
	if encryptedValue == "" {
		return "", nil
	}
	encryptedBytes, err := hex.DecodeString(encryptedValue)
	if err != nil {
		return "", fmt.Errorf("could not decode encrypted value: %w", err)
	}
	decryptedValue, err := h.keeper.Decrypt(ctx, encryptedBytes)
	if err != nil {
		return "", fmt.Errorf("could not decrypt value: %w", err)
	}
	return string(decryptedValue), nil
}

func (h *WebhookDeliveryHandler) decryptWebhookHeaders(ctx context.Context, encryptedHeaders sql.NullString) ([]webhooks.Header, error) {
	if !encryptedHeaders.Valid || encryptedHeaders.String == "" {
		return nil, nil
	}
	raw, err := h.decryptWebhookValue(ctx, encryptedHeaders.String)
	if err != nil {
		return nil, err
	}
	var headers []webhooks.Header
	if err := json.Unmarshal([]byte(raw), &headers); err != nil {
		return nil, err
	}
	return headers, nil
}
