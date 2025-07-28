package handlers

import (
	"fmt"
	"net/http"

	"github.com/cvhariharan/flowctl/internal/core/models"
	"github.com/labstack/echo/v4"
)

func (h *Handler) HandleCreateNamespace(c echo.Context) error {
	var req NamespaceReq
	if err := c.Bind(&req); err != nil {
		return wrapError(http.StatusBadRequest, "could not decode request", err, nil)
	}

	if err := h.validate.Struct(req); err != nil {
		return wrapError(http.StatusBadRequest, fmt.Sprintf("request validation failed: %s", formatValidationErrors(err)), err, nil)
	}

	namespace := &models.Namespace{
		Name: req.Name,
	}

	created, err := h.co.CreateNamespace(c.Request().Context(), namespace)
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not create namespace", err, nil)
	}

	return c.JSON(http.StatusCreated, coreNamespaceToNamespaceResp(created))
}

func (h *Handler) HandleGetNamespace(c echo.Context) error {
	namespaceID := c.Param("namespaceID")
	if namespaceID == "" {
		return wrapError(http.StatusBadRequest, "namespace ID cannot be empty", nil, nil)
	}

	namespace, err := h.co.GetNamespaceByID(c.Request().Context(), namespaceID)
	if err != nil {
		return wrapError(http.StatusNotFound, "namespace not found", err, nil)
	}

	return c.JSON(http.StatusOK, coreNamespaceToNamespaceResp(namespace))
}

func (h *Handler) HandleListNamespaces(c echo.Context) error {
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

	userInfo, err := h.getUserInfo(c)
	if err != nil {
		return wrapError(http.StatusUnauthorized, "could not get user info", err, nil)
	}

	namespaces, pageCount, totalCount, err := h.co.ListNamespaces(c.Request().Context(), userInfo.ID, req.Count, req.Count*req.Page)
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not list namespaces", err, nil)
	}

	return c.JSON(http.StatusOK, NamespacesPaginateResponse{
		Namespaces: coreNamespaceArrayToNamespaceRespArray(namespaces),
		PageCount:  pageCount,
		TotalCount: totalCount,
	})
}

func (h *Handler) HandleUpdateNamespace(c echo.Context) error {
	namespaceID := c.Param("namespaceID")
	if namespaceID == "" {
		return wrapError(http.StatusBadRequest, "namespace ID cannot be empty", nil, nil)
	}

	var req NamespaceReq
	if err := c.Bind(&req); err != nil {
		return wrapError(http.StatusBadRequest, "could not decode request", err, nil)
	}

	if err := h.validate.Struct(req); err != nil {
		return wrapError(http.StatusBadRequest, fmt.Sprintf("request validation failed: %s", formatValidationErrors(err)), err, nil)
	}

	namespace := &models.Namespace{
		Name: req.Name,
	}

	updated, err := h.co.UpdateNamespace(c.Request().Context(), namespaceID, *namespace)
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not update namespace", err, nil)
	}

	return c.JSON(http.StatusOK, coreNamespaceToNamespaceResp(updated))
}

func (h *Handler) HandleDeleteNamespace(c echo.Context) error {
	namespaceID := c.Param("namespaceID")
	if namespaceID == "" {
		return wrapError(http.StatusBadRequest, "namespace ID cannot be empty", nil, nil)
	}

	err := h.co.DeleteNamespace(c.Request().Context(), namespaceID)
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not delete namespace", err, nil)
	}

	return c.NoContent(http.StatusOK)
}

func (h *Handler) HandleAddNamespaceMember(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(http.StatusBadRequest, "could not get namespace", nil, nil)
	}

	var req NamespaceMemberReq
	if err := c.Bind(&req); err != nil {
		return wrapError(http.StatusBadRequest, "could not decode request", err, nil)
	}

	if err := h.validate.Struct(req); err != nil {
		return wrapError(http.StatusBadRequest, fmt.Sprintf("request validation failed: %s", formatValidationErrors(err)), err, nil)
	}

	role := models.NamespaceRole(req.Role)
	err := h.co.AssignNamespaceRole(c.Request().Context(), req.SubjectID, req.SubjectType, namespace, role)
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not assign role", err, nil)
	}

	return c.NoContent(http.StatusOK)
}

func (h *Handler) HandleGetNamespaceMembers(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(http.StatusBadRequest, "could not get namespace", nil, nil)
	}

	members, err := h.co.GetNamespaceMembers(c.Request().Context(), namespace)
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not get namespace members", err, nil)
	}

	return c.JSON(http.StatusOK, coreNamespaceMembersToResp(members))
}

func (h *Handler) HandleRemoveNamespaceMember(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(http.StatusBadRequest, "could not get namespace", nil, nil)
	}

	membershipID := c.Param("membershipID")
	if membershipID == "" {
		return wrapError(http.StatusBadRequest, "subject ID cannot be empty", nil, nil)
	}

	err := h.co.RemoveNamespaceMember(c.Request().Context(), membershipID, namespace)
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not remove namespace member", err, nil)
	}

	return c.NoContent(http.StatusOK)
}
