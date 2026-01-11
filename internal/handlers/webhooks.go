package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/cvhariharan/flowctl/internal/core/models"
	"github.com/cvhariharan/flowctl/internal/webhooks"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

const maskedValue = "********"

func (h *Handler) HandleListWebhooks(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}

	webhooksList, err := h.co.ListNamespaceWebhooks(c.Request().Context(), namespace)
	if err != nil {
		return wrapError(ErrOperationFailed, "could not list webhooks", err, nil)
	}

	resp := make([]WebhookListItem, 0, len(webhooksList))
	for _, hook := range webhooksList {
		resp = append(resp, WebhookListItem{
			ID:          hook.ID,
			Name:        hook.Name,
			Type:        hook.Type,
			Description: hook.Description,
			URLMasked:   maskWebhookURL(hook.URL),
			IsActive:    hook.IsActive,
			CreatedAt:   hook.CreatedAt,
			UpdatedAt:   hook.UpdatedAt,
		})
	}

	return c.JSON(http.StatusOK, WebhookListResp{Webhooks: resp})
}

func (h *Handler) HandleCreateWebhook(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}

	var req WebhookCreateReq
	if err := c.Bind(&req); err != nil {
		return wrapError(ErrInvalidInput, "could not decode request", err, nil)
	}

	if err := h.validate.Struct(req); err != nil {
		return wrapError(ErrValidationFailed, fmt.Sprintf("request validation failed: %s", formatValidationErrors(err)), err, nil)
	}

	if err := validateWebhookTemplate(req.Template.Body, h.config.App.RootURL); err != nil {
		return wrapError(ErrValidationFailed, fmt.Sprintf("template validation failed: %s", err.Error()), err, nil)
	}

	headers := toWebhookHeaders(req.Headers)
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	created, err := h.co.CreateNamespaceWebhook(c.Request().Context(), models.WebhookDestination{
		Name:        req.Name,
		Description: req.Description,
		Type:        req.Type,
		URL:         req.URL,
		Headers:     headers,
		ContentType: req.ContentType,
		Template: models.WebhookTemplate{
			Format: req.Template.Format,
			Body:   req.Template.Body,
		},
		IsActive: isActive,
	}, namespace)
	if err != nil {
		return wrapError(ErrOperationFailed, "could not create webhook", err, nil)
	}

	return c.JSON(http.StatusCreated, buildWebhookResp(created, true))
}

func (h *Handler) HandleGetWebhook(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}

	var req WebhookGetReq
	if err := c.Bind(&req); err != nil {
		return wrapError(ErrInvalidInput, "could not decode request", err, nil)
	}

	if err := h.validate.Struct(req); err != nil {
		return wrapError(ErrValidationFailed, fmt.Sprintf("request validation failed: %s", formatValidationErrors(err)), err, nil)
	}

	webhook, err := h.co.GetNamespaceWebhookByID(c.Request().Context(), req.WebhookID, namespace)
	if err != nil {
		return wrapError(ErrResourceNotFound, "webhook not found", err, nil)
	}

	return c.JSON(http.StatusOK, buildWebhookResp(webhook, true))
}

func (h *Handler) HandleUpdateWebhook(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}

	var req WebhookUpdateReq
	if err := c.Bind(&req); err != nil {
		return wrapError(ErrInvalidInput, "could not decode request", err, nil)
	}

	if err := h.validate.Struct(req); err != nil {
		return wrapError(ErrValidationFailed, fmt.Sprintf("request validation failed: %s", formatValidationErrors(err)), err, nil)
	}

	if err := validateWebhookTemplate(req.Template.Body, h.config.App.RootURL); err != nil {
		return wrapError(ErrValidationFailed, fmt.Sprintf("template validation failed: %s", err.Error()), err, nil)
	}

	var headers *[]models.WebhookHeader
	if req.Headers != nil {
		converted := toWebhookHeaders(*req.Headers)
		headers = &converted
	}

	updated, err := h.co.UpdateNamespaceWebhook(c.Request().Context(), req.WebhookID, models.WebhookDestinationUpdate{
		Name:        req.Name,
		Description: req.Description,
		Type:        req.Type,
		URL:         req.URL,
		Headers:     headers,
		ContentType: req.ContentType,
		Template: models.WebhookTemplate{
			Format: req.Template.Format,
			Body:   req.Template.Body,
		},
		IsActive: req.IsActive,
	}, namespace)
	if err != nil {
		return wrapError(ErrOperationFailed, "could not update webhook", err, nil)
	}

	return c.JSON(http.StatusOK, buildWebhookResp(updated, true))
}

func (h *Handler) HandleDeleteWebhook(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}

	var req WebhookGetReq
	if err := c.Bind(&req); err != nil {
		return wrapError(ErrInvalidInput, "could not decode request", err, nil)
	}

	if err := h.validate.Struct(req); err != nil {
		return wrapError(ErrValidationFailed, fmt.Sprintf("request validation failed: %s", formatValidationErrors(err)), err, nil)
	}

	if err := h.co.DeleteNamespaceWebhook(c.Request().Context(), req.WebhookID, namespace); err != nil {
		return wrapError(ErrOperationFailed, "could not delete webhook", err, nil)
	}

	return c.NoContent(http.StatusOK)
}

func (h *Handler) HandleDuplicateWebhook(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}

	webhookID := c.Param("webhookID")
	if webhookID == "" {
		return wrapError(ErrRequiredFieldMissing, "webhook ID is required", nil, nil)
	}

	var dupReq WebhookDuplicateReq
	if err := c.Bind(&dupReq); err != nil {
		return wrapError(ErrInvalidInput, "could not decode request", err, nil)
	}
	if err := h.validate.Struct(dupReq); err != nil {
		return wrapError(ErrValidationFailed, fmt.Sprintf("request validation failed: %s", formatValidationErrors(err)), err, nil)
	}

	created, err := h.co.DuplicateNamespaceWebhook(c.Request().Context(), webhookID, dupReq.Name, dupReq.Description, namespace)
	if err != nil {
		return wrapError(ErrOperationFailed, "could not duplicate webhook", err, nil)
	}

	return c.JSON(http.StatusCreated, buildWebhookResp(created, true))
}

func (h *Handler) HandleTestWebhook(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}

	var req WebhookGetReq
	if err := c.Bind(&req); err != nil {
		return wrapError(ErrInvalidInput, "could not decode request", err, nil)
	}

	if err := h.validate.Struct(req); err != nil {
		return wrapError(ErrValidationFailed, fmt.Sprintf("request validation failed: %s", formatValidationErrors(err)), err, nil)
	}

	webhook, err := h.co.GetNamespaceWebhookByID(c.Request().Context(), req.WebhookID, namespace)
	if err != nil {
		return wrapError(ErrResourceNotFound, "webhook not found", err, nil)
	}

	body, err := webhooks.RenderTemplate(webhook.Template.Body, webhooks.SampleTemplateData(h.config.App.RootURL))
	if err != nil {
		return wrapError(ErrValidationFailed, fmt.Sprintf("template render failed: %s", err.Error()), err, nil)
	}

	sender := webhooks.NewSender(h.logger.WithGroup("webhook_test"))
	_, err = sender.Send(c.Request().Context(), webhooks.Request{
		URL:         webhook.URL,
		Body:        body,
		ContentType: webhook.ContentType,
		Headers:     toWebhookHeadersForSend(webhook.Headers),
		Event:       "test",
		DeliveryID:  uuid.NewString(),
	})
	if err != nil {
		return wrapError(ErrOperationFailed, "failed to send test webhook", err, nil)
	}

	return c.JSON(http.StatusOK, WebhookTestResp{Message: "Test sent"})
}

func validateWebhookTemplate(body string, rootURL string) error {
	_, err := webhooks.RenderTemplate(body, webhooks.SampleTemplateData(rootURL))
	return err
}

func maskWebhookURL(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "***"
	}
	trimmedPath := strings.Trim(parsed.Path, "/")
	if trimmedPath == "" {
		return fmt.Sprintf("%s://%s/***", parsed.Scheme, parsed.Host)
	}
	segments := strings.Split(trimmedPath, "/")
	segment := segments[0]
	return fmt.Sprintf("%s://%s/%s/***", parsed.Scheme, parsed.Host, segment)
}

func buildWebhookResp(webhook models.WebhookDestination, includeHeaders bool) WebhookResp {
	var headers []WebhookHeaderResp
	if includeHeaders {
		headers = maskWebhookHeaders(webhook.Headers)
	}

	return WebhookResp{
		ID:          webhook.ID,
		Name:        webhook.Name,
		Type:        webhook.Type,
		Description: webhook.Description,
		URLMasked:   maskWebhookURL(webhook.URL),
		ContentType: webhook.ContentType,
		Headers:     headers,
		Template: WebhookTemplateResp{
			Format: webhook.Template.Format,
			Body:   webhook.Template.Body,
		},
		IsActive:  webhook.IsActive,
		CreatedAt: webhook.CreatedAt,
		UpdatedAt: webhook.UpdatedAt,
	}
}

func maskWebhookHeaders(headers []models.WebhookHeader) []WebhookHeaderResp {
	if len(headers) == 0 {
		return nil
	}
	resp := make([]WebhookHeaderResp, 0, len(headers))
	for _, header := range headers {
		value := ""
		if strings.TrimSpace(header.Value) != "" {
			value = maskedValue
		}
		resp = append(resp, WebhookHeaderResp{
			Key:   header.Key,
			Value: value,
		})
	}
	return resp
}

func toWebhookHeaders(headers []WebhookHeaderReq) []models.WebhookHeader {
	if len(headers) == 0 {
		return nil
	}
	resp := make([]models.WebhookHeader, 0, len(headers))
	for _, header := range headers {
		resp = append(resp, models.WebhookHeader{
			Key:   strings.TrimSpace(header.Key),
			Value: header.Value,
		})
	}
	return resp
}

func toWebhookHeadersForSend(headers []models.WebhookHeader) []webhooks.Header {
	if len(headers) == 0 {
		return nil
	}
	resp := make([]webhooks.Header, 0, len(headers))
	for _, header := range headers {
		resp = append(resp, webhooks.Header{
			Key:   header.Key,
			Value: header.Value,
		})
	}
	return resp
}
