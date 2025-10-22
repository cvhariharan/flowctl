package handlers

import (
	"fmt"
	"net/http"

	"github.com/cvhariharan/flowctl/internal/core/models"
	"github.com/labstack/echo/v4"
)

func (h *Handler) HandleCreateFlowSecret(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}

	var req FlowSecretReq
	if err := c.Bind(&req); err != nil {
		return wrapError(ErrInvalidInput, "could not decode request", err, nil)
	}

	if err := h.validate.Struct(req); err != nil {
		return wrapError(ErrValidationFailed, fmt.Sprintf("request validation failed: %s", formatValidationErrors(err)), err, nil)
	}

	secret := models.FlowSecret{
		Key:         req.Key,
		Value:       req.Value,
		Description: req.Description,
	}

	created, err := h.co.CreateFlowSecret(c.Request().Context(), req.FlowID, secret, namespace)
	if err != nil {
		return wrapError(ErrOperationFailed, "could not create flow secret", err, nil)
	}

	return c.JSON(http.StatusCreated, coreFlowSecretToFlowSecretResp(created))
}

func (h *Handler) HandleGetFlowSecret(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}

	var req FlowSecretGetReq
	if err := c.Bind(&req); err != nil {
		return wrapError(ErrInvalidInput, "could not decode request", err, nil)
	}

	if err := h.validate.Struct(req); err != nil {
		return wrapError(ErrValidationFailed, fmt.Sprintf("request validation failed: %s", formatValidationErrors(err)), err, nil)
	}

	secret, err := h.co.GetFlowSecretByID(c.Request().Context(), req.SecretID, namespace)
	if err != nil {
		return wrapError(ErrResourceNotFound, "secret not found", err, nil)
	}

	return c.JSON(http.StatusOK, coreFlowSecretToFlowSecretResp(secret))
}

func (h *Handler) HandleListFlowSecrets(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}

	var req FlowSecretsListReq
	if err := c.Bind(&req); err != nil {
		return wrapError(ErrInvalidInput, "could not decode request", err, nil)
	}

	if err := h.validate.Struct(req); err != nil {
		return wrapError(ErrValidationFailed, fmt.Sprintf("request validation failed: %s", formatValidationErrors(err)), err, nil)
	}

	secrets, err := h.co.ListFlowSecrets(c.Request().Context(), req.FlowID, namespace)
	if err != nil {
		return wrapError(ErrOperationFailed, "could not list flow secrets", err, nil)
	}

	resp := make([]FlowSecretResp, len(secrets))
	for i, secret := range secrets {
		resp[i] = coreFlowSecretToFlowSecretResp(secret)
	}

	return c.JSON(http.StatusOK, resp)
}

func (h *Handler) HandleUpdateFlowSecret(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}

	var req FlowSecretUpdateReq
	if err := c.Bind(&req); err != nil {
		return wrapError(ErrInvalidInput, "could not decode request", err, nil)
	}

	if err := h.validate.Struct(req); err != nil {
		return wrapError(ErrValidationFailed, fmt.Sprintf("request validation failed: %s", formatValidationErrors(err)), err, nil)
	}

	secret := models.FlowSecret{
		Key:         req.Key,
		Value:       req.Value,
		Description: req.Description,
	}

	updated, err := h.co.UpdateFlowSecret(c.Request().Context(), req.SecretID, secret, namespace)
	if err != nil {
		return wrapError(ErrOperationFailed, "could not update flow secret", err, nil)
	}

	return c.JSON(http.StatusOK, coreFlowSecretToFlowSecretResp(updated))
}

func (h *Handler) HandleDeleteFlowSecret(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}

	var req FlowSecretGetReq
	if err := c.Bind(&req); err != nil {
		return wrapError(ErrInvalidInput, "could not decode request", err, nil)
	}

	if err := h.validate.Struct(req); err != nil {
		return wrapError(ErrValidationFailed, fmt.Sprintf("request validation failed: %s", formatValidationErrors(err)), err, nil)
	}

	err := h.co.DeleteFlowSecret(c.Request().Context(), req.SecretID, namespace)
	if err != nil {
		return wrapError(ErrOperationFailed, "could not delete flow secret", err, nil)
	}

	return c.NoContent(http.StatusOK)
}
