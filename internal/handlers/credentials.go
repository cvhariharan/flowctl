package handlers

import (
	"fmt"
	"net/http"

	"github.com/cvhariharan/autopilot/internal/core/models"
	"github.com/labstack/echo/v4"
)

func (h *Handler) HandleCreateCredential(c echo.Context) error {
	var req CredentialReq
	if err := c.Bind(&req); err != nil {
		return wrapError(http.StatusBadRequest, "could not decode request", err, nil)
	}

	if err := h.validate.Struct(req); err != nil {
		return wrapError(http.StatusBadRequest, fmt.Sprintf("request validation failed: %s", formatValidationErrors(err)), err, nil)
	}

	if req.Password != "" && req.PrivateKey != "" {
		return wrapError(http.StatusBadRequest, "cannot set both password and private key", nil, nil)
	}

	cred := &models.Credential{
		Name:       req.Name,
		PrivateKey: req.PrivateKey,
		Password:   req.Password,
	}

	created, err := h.co.CreateCredential(c.Request().Context(), cred)
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not create credential", err, nil)
	}

	return c.JSON(http.StatusCreated, coreCredentialToCredentialResp(created))
}

func (h *Handler) HandleGetCredential(c echo.Context) error {
	credID := c.Param("credID")
	if credID == "" {
		return wrapError(http.StatusBadRequest, "credential ID cannot be empty", nil, nil)
	}

	cred, err := h.co.GetCredentialByID(c.Request().Context(), credID)
	if err != nil {
		return wrapError(http.StatusNotFound, "credential not found", err, nil)
	}

	return c.JSON(http.StatusOK, coreCredentialToCredentialResp(cred))
}

func (h *Handler) HandleListCredentials(c echo.Context) error {
	var req PaginateRequest
	if err := c.Bind(&req); err != nil {
		return wrapError(http.StatusBadRequest, "could not decode request", err, nil)
	}

	if req.Page < 0 || req.Count < 0 {
		return wrapError(http.StatusBadRequest, "invalid pagination parameters", nil, nil)
	}

	if req.Page > 0 {
		req.Page -= 1
	}

	if req.Count == 0 {
		req.Count = CountPerPage
	}

	creds, pageCount, totalCount, err := h.co.ListCredentials(c.Request().Context(), req.Count, req.Count*req.Page)
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not list credentials", err, nil)
	}

	return c.JSON(http.StatusOK, CredentialsPaginateResponse{
		Credentials: coreCredentialArrayToCredentialRespArray(creds),
		PageCount:   pageCount,
		TotalCount:  totalCount,
	})
}

func (h *Handler) HandleUpdateCredential(c echo.Context) error {
	credID := c.Param("credID")
	if credID == "" {
		return wrapError(http.StatusBadRequest, "credential ID cannot be empty", nil, nil)
	}

	var req CredentialReq
	if err := c.Bind(&req); err != nil {
		return wrapError(http.StatusBadRequest, "could not decode request", err, nil)
	}

	if err := h.validate.Struct(req); err != nil {
		return wrapError(http.StatusBadRequest, fmt.Sprintf("request validation failed: %s", formatValidationErrors(err)), err, nil)
	}

	cred := &models.Credential{
		Name:       req.Name,
		PrivateKey: req.PrivateKey,
		Password:   req.Password,
	}

	updated, err := h.co.UpdateCredential(c.Request().Context(), credID, cred)
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not update credential", err, nil)
	}

	return c.JSON(http.StatusOK, coreCredentialToCredentialResp(updated))
}

func (h *Handler) HandleDeleteCredential(c echo.Context) error {
	credID := c.Param("credID")
	if credID == "" {
		return wrapError(http.StatusBadRequest, "credential ID cannot be empty", nil, nil)
	}

	err := h.co.DeleteCredential(c.Request().Context(), credID)
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not delete credential", err, nil)
	}

	return c.NoContent(http.StatusOK)
}
