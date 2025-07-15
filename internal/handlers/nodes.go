package handlers

import (
	"fmt"
	"net/http"

	"github.com/cvhariharan/autopilot/internal/core/models"
	"github.com/labstack/echo/v4"
)

func (h *Handler) HandleCreateNode(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(http.StatusBadRequest, "could not get namespace", nil, nil)
	}

	var req NodeReq
	if err := c.Bind(&req); err != nil {
		return wrapError(http.StatusBadRequest, "could not decode request", err, nil)
	}

	if err := h.validate.Struct(req); err != nil {
		return wrapError(http.StatusBadRequest, fmt.Sprintf("request validation failed: %s", formatValidationErrors(err)), err, nil)
	}

	node := &models.Node{
		Name:     req.Name,
		Hostname: req.Hostname,
		Port:     req.Port,
		Username: req.Username,
		OSFamily: req.OSFamily,
		Tags:     req.Tags,
		Auth: models.NodeAuth{
			Method:       models.AuthMethod(req.Auth.Method),
			CredentialID: req.Auth.CredentialID,
		},
	}

	created, err := h.co.CreateNode(c.Request().Context(), node, namespace)
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not create node", err, nil)
	}

	return c.JSON(http.StatusCreated, coreNodeToNodeResp(created))
}

func (h *Handler) HandleGetNode(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(http.StatusBadRequest, "could not get namespace", nil, nil)
	}

	nodeID := c.Param("nodeID")
	if nodeID == "" {
		return wrapError(http.StatusBadRequest, "node ID cannot be empty", nil, nil)
	}

	node, err := h.co.GetNodeByID(c.Request().Context(), nodeID, namespace)
	if err != nil {
		return wrapError(http.StatusNotFound, "node not found", err, nil)
	}

	return c.JSON(http.StatusOK, coreNodeToNodeResp(node))
}

func (h *Handler) HandleListNodes(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(http.StatusBadRequest, "could not get namespace", nil, nil)
	}

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

	nodes, pageCount, totalCount, err := h.co.ListNodes(c.Request().Context(), req.Count, req.Count*req.Page, namespace)
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not list nodes", err, nil)
	}

	return c.JSON(http.StatusOK, NodesPaginateResponse{
		Nodes:      coreNodeArrayToNodeRespArray(nodes),
		PageCount:  pageCount,
		TotalCount: totalCount,
	})
}

func (h *Handler) HandleUpdateNode(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(http.StatusBadRequest, "could not get namespace", nil, nil)
	}

	nodeID := c.Param("nodeID")
	if nodeID == "" {
		return wrapError(http.StatusBadRequest, "node ID cannot be empty", nil, nil)
	}

	var req NodeReq
	if err := c.Bind(&req); err != nil {
		return wrapError(http.StatusBadRequest, "could not decode request", err, nil)
	}

	if err := h.validate.Struct(req); err != nil {
		return wrapError(http.StatusBadRequest, fmt.Sprintf("request validation failed: %s", formatValidationErrors(err)), err, nil)
	}

	node := &models.Node{
		Name:     req.Name,
		Hostname: req.Hostname,
		Port:     req.Port,
		Username: req.Username,
		OSFamily: req.OSFamily,
		Tags:     req.Tags,
		Auth: models.NodeAuth{
			Method:       models.AuthMethod(req.Auth.Method),
			CredentialID: req.Auth.CredentialID,
		},
	}

	updated, err := h.co.UpdateNode(c.Request().Context(), nodeID, node, namespace)
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not update node", err, nil)
	}

	return c.JSON(http.StatusOK, coreNodeToNodeResp(updated))
}

func (h *Handler) HandleDeleteNode(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(http.StatusBadRequest, "could not get namespace", nil, nil)
	}

	nodeID := c.Param("nodeID")
	if nodeID == "" {
		return wrapError(http.StatusBadRequest, "node ID cannot be empty", nil, nil)
	}

	err := h.co.DeleteNode(c.Request().Context(), nodeID, namespace)
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not delete node", err, nil)
	}

	return c.NoContent(http.StatusOK)
}
