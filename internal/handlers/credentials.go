package handlers

import (
	"fmt"
	"net/http"

	"github.com/cvhariharan/flowctl/internal/core/models"
	"github.com/labstack/echo/v4"
)

func (h *Handler) HandleCreateCredential(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}

	var req CredentialReq
	if err := c.Bind(&req); err != nil {
		return wrapError(ErrInvalidInput, "could not decode request", err, nil)
	}

	if err := h.validate.Struct(req); err != nil {
		return wrapError(ErrValidationFailed, fmt.Sprintf("request validation failed: %s", formatValidationErrors(err)), err, nil)
	}

	cred := &models.Credential{
		Name:    req.Name,
		KeyType: req.KeyType,
		KeyData: req.KeyData,
	}

	created, err := h.co.CreateCredential(c.Request().Context(), cred, namespace)
	if err != nil {
		return wrapError(ErrOperationFailed, "could not create credential", err, nil)
	}

	return c.JSON(http.StatusCreated, coreCredentialToCredentialResp(created))
}

func (h *Handler) HandleGetCredential(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}

	var req CredentialGetReq
	if err := c.Bind(&req); err != nil {
		return wrapError(ErrInvalidInput, "could not decode request", err, nil)
	}

	if err := h.validate.Struct(req); err != nil {
		return wrapError(ErrValidationFailed, fmt.Sprintf("request validation failed: %s", formatValidationErrors(err)), err, nil)
	}

	cred, err := h.co.GetCredentialByID(c.Request().Context(), req.CredID, namespace)
	if err != nil {
		return wrapError(ErrResourceNotFound, "credential not found", err, nil)
	}

	return c.JSON(http.StatusOK, coreCredentialToCredentialResp(cred))
}

func (h *Handler) HandleListCredentials(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}

	var req PaginateRequest
	if err := c.Bind(&req); err != nil {
		return wrapError(ErrInvalidInput, "could not decode request", err, nil)
	}

	if req.Page < 0 || req.Count < 0 {
		return wrapError(ErrInvalidPagination, "invalid pagination parameters", nil, nil)
	}

	if req.Page > 0 {
		req.Page -= 1
	}

	if req.Count == 0 {
		req.Count = CountPerPage
	}

	creds, pageCount, totalCount, err := h.co.ListCredentials(c.Request().Context(), req.Count, req.Count*req.Page, namespace)
	if err != nil {
		return wrapError(ErrOperationFailed, "could not list credentials", err, nil)
	}

	return c.JSON(http.StatusOK, CredentialsPaginateResponse{
		Credentials: coreCredentialArrayToCredentialRespArray(creds),
		PageCount:   pageCount,
		TotalCount:  totalCount,
	})
}

func (h *Handler) HandleUpdateCredential(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}

	var req CredentialUpdateReq
	if err := c.Bind(&req); err != nil {
		return wrapError(ErrInvalidInput, "could not decode request", err, nil)
	}

	if err := h.validate.Struct(req); err != nil {
		return wrapError(ErrValidationFailed, fmt.Sprintf("request validation failed: %s", formatValidationErrors(err)), err, nil)
	}

	cred := &models.Credential{
		Name:    req.Name,
		KeyType: req.KeyType,
		KeyData: req.KeyData,
	}

	updated, err := h.co.UpdateCredential(c.Request().Context(), req.CredID, cred, namespace)
	if err != nil {
		return wrapError(ErrOperationFailed, "could not update credential", err, nil)
	}

	return c.JSON(http.StatusOK, coreCredentialToCredentialResp(updated))
}

func (h *Handler) HandleDeleteCredential(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}

	var req CredentialGetReq
	if err := c.Bind(&req); err != nil {
		return wrapError(ErrInvalidInput, "could not decode request", err, nil)
	}

	if err := h.validate.Struct(req); err != nil {
		return wrapError(ErrValidationFailed, fmt.Sprintf("request validation failed: %s", formatValidationErrors(err)), err, nil)
	}

	err := h.co.DeleteCredential(c.Request().Context(), req.CredID, namespace)
	if err != nil {
		return wrapError(ErrOperationFailed, "could not delete credential", err, nil)
	}

	return c.NoContent(http.StatusOK)
}
