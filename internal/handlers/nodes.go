package handlers

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/cvhariharan/flowctl/internal/core/models"
	"github.com/labstack/echo/v4"
)

const maxCSVImportSize = 1 << 20 // 1MB

func (h *Handler) HandleCreateNode(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}

	var req NodeReq
	if err := c.Bind(&req); err != nil {
		return wrapError(ErrInvalidInput, "could not decode request", err, nil)
	}

	if err := h.validate.Struct(req); err != nil {
		return wrapError(ErrValidationFailed, fmt.Sprintf("request validation failed: %s", formatValidationErrors(err)), err, nil)
	}

	if req.Name == "local" || req.Name == "uploads" {
		return wrapError(ErrValidationFailed, fmt.Sprintf("node name cannot be %s", req.Name), fmt.Errorf("node name cannot be %s", req.Name), nil)
	}

	node := &models.Node{
		Name:     req.Name,
		Hostname: req.Hostname,
		Port:     req.Port,
		Username: req.Username,
		// Only linux is supported right now
		OSFamily:       "linux",
		ConnectionType: req.ConnectionType,
		Tags:           req.Tags,
		Auth: models.NodeAuth{
			Method:       models.AuthMethod(req.Auth.Method),
			CredentialID: req.Auth.CredentialID,
		},
	}

	created, err := h.co.CreateNode(c.Request().Context(), node, namespace)
	if err != nil {
		return wrapError(ErrOperationFailed, "could not create node", err, nil)
	}

	return c.JSON(http.StatusCreated, coreNodeToNodeResp(created))
}

func (h *Handler) HandleGetNode(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}

	nodeID := c.Param("nodeID")
	if nodeID == "" {
		return wrapError(ErrRequiredFieldMissing, "node ID cannot be empty", nil, nil)
	}

	node, err := h.co.GetNodeByID(c.Request().Context(), nodeID, namespace)
	if err != nil {
		return wrapError(ErrResourceNotFound, "node not found", err, nil)
	}

	return c.JSON(http.StatusOK, coreNodeToNodeResp(node))
}

func (h *Handler) HandleListNodes(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}
	return h.listNodes(c, namespace)
}

// HandleListNodesForFlow lists nodes scoped to a flow the caller can execute.
// Used by the trigger UI so that users with flow:execute (but not node:view)
// can still pick hosts for a node-typed flow input.
func (h *Handler) HandleListNodesForFlow(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}
	return h.listNodes(c, namespace)
}

func (h *Handler) listNodes(c echo.Context, namespace string) error {
	var req NodePaginateRequest
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

	nodes, pageCount, totalCount, err := h.co.SearchNodes(c.Request().Context(), req.Filter, req.Tags, req.Count, req.Count*req.Page, namespace)
	if err != nil {
		return wrapError(ErrOperationFailed, "could not list nodes", err, nil)
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
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}

	nodeID := c.Param("nodeID")
	if nodeID == "" {
		return wrapError(ErrRequiredFieldMissing, "node ID cannot be empty", nil, nil)
	}

	var req NodeReq
	if err := c.Bind(&req); err != nil {
		return wrapError(ErrInvalidInput, "could not decode request", err, nil)
	}

	if err := h.validate.Struct(req); err != nil {
		return wrapError(ErrValidationFailed, fmt.Sprintf("request validation failed: %s", formatValidationErrors(err)), err, nil)
	}

	node := &models.Node{
		Name:     req.Name,
		Hostname: req.Hostname,
		Port:     req.Port,
		Username: req.Username,
		// Only linux is supported right now
		OSFamily:       "linux",
		ConnectionType: req.ConnectionType,
		Tags:           req.Tags,
		Auth: models.NodeAuth{
			Method:       models.AuthMethod(req.Auth.Method),
			CredentialID: req.Auth.CredentialID,
		},
	}

	updated, err := h.co.UpdateNode(c.Request().Context(), nodeID, node, namespace)
	if err != nil {
		return wrapError(ErrOperationFailed, "could not update node", err, nil)
	}

	return c.JSON(http.StatusOK, coreNodeToNodeResp(updated))
}

func (h *Handler) HandleDeleteNode(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}

	nodeID := c.Param("nodeID")
	if nodeID == "" {
		return wrapError(ErrRequiredFieldMissing, "node ID cannot be empty", nil, nil)
	}

	err := h.co.DeleteNode(c.Request().Context(), nodeID, namespace)
	if err != nil {
		return wrapError(ErrOperationFailed, "could not delete node", err, nil)
	}

	return c.NoContent(http.StatusOK)
}

func (h *Handler) HandleGetNodeStats(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}

	stats, err := h.co.GetNodeStats(c.Request().Context(), namespace)
	if err != nil {
		return wrapError(ErrOperationFailed, "could not get node stats", err, nil)
	}

	return c.JSON(http.StatusOK, NodeStatsResp{
		TotalHosts: stats.TotalHosts,
		SSHHosts:   stats.SSHHosts,
		QSSHHosts:  stats.QSSHHosts,
	})
}

func (h *Handler) HandleImportNodes(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}

	c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, maxCSVImportSize+importBodyOverhead)

	file, err := c.FormFile("file")
	if err != nil {
		return wrapError(ErrInvalidInput, "csv file is required", err, nil)
	}

	credentialID := c.FormValue("credential_id")
	if credentialID == "" {
		return wrapError(ErrInvalidInput, "credential_id is required", nil, nil)
	}

	credential, err := h.co.GetCredentialByID(c.Request().Context(), credentialID, namespace)
	if err != nil {
		return wrapError(ErrResourceNotFound, "credential not found", err, nil)
	}

	src, err := file.Open()
	if err != nil {
		return wrapError(ErrInvalidInput, "could not open csv file", err, nil)
	}
	defer src.Close()

	reader := csv.NewReader(src)
	reader.TrimLeadingSpace = true

	// read and validate header row
	header, err := reader.Read()
	if err != nil {
		return wrapError(ErrInvalidInput, "could not read csv header", err, nil)
	}
	expected := []string{"name", "hostname", "port", "username", "connection_type", "tags"}
	if len(header) < len(expected) {
		return wrapError(ErrInvalidInput, fmt.Sprintf("csv must have columns: %s", strings.Join(expected, ",")), nil, nil)
	}
	for i, col := range expected {
		if strings.TrimSpace(strings.ToLower(header[i])) != col {
			return wrapError(ErrInvalidInput, fmt.Sprintf("expected column %d to be %q, got %q", i+1, col, header[i]), nil, nil)
		}
	}

	var importErrors []NodeImportError
	imported := 0
	rowNum := 1

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		rowNum++
		if err != nil {
			importErrors = append(importErrors, NodeImportError{Row: rowNum, Error: fmt.Sprintf("could not parse row: %s", err.Error())})
			continue
		}
		if len(record) < len(expected) {
			importErrors = append(importErrors, NodeImportError{Row: rowNum, Error: "row has too few columns"})
			continue
		}

		name := strings.TrimSpace(record[0])
		hostname := strings.TrimSpace(record[1])
		portStr := strings.TrimSpace(record[2])
		username := strings.TrimSpace(record[3])
		connType := strings.TrimSpace(record[4])
		tagsRaw := strings.TrimSpace(record[5])

		port, err := strconv.Atoi(portStr)
		if err != nil || port < 1 || port > 65535 {
			importErrors = append(importErrors, NodeImportError{Row: rowNum, Name: name, Error: fmt.Sprintf("invalid port %q", portStr)})
			continue
		}

		var tags []string
		if tagsRaw != "" {
			for _, t := range strings.Split(tagsRaw, ",") {
				if trimmed := strings.TrimSpace(t); trimmed != "" {
					tags = append(tags, trimmed)
				}
			}
		}

		if name == "local" || name == "uploads" {
			importErrors = append(importErrors, NodeImportError{Row: rowNum, Name: name, Error: fmt.Sprintf("node name cannot be %s", name)})
			continue
		}

		req := NodeReq{
			Name:           name,
			Hostname:       hostname,
			Port:           port,
			Username:       username,
			ConnectionType: connType,
			Tags:           tags,
			Auth: NodeAuth{
				CredentialID: credentialID,
				Method:       credential.KeyType,
			},
		}

		if err := h.validate.Struct(req); err != nil {
			importErrors = append(importErrors, NodeImportError{Row: rowNum, Name: name, Error: formatValidationErrors(err)})
			continue
		}

		node := &models.Node{
			Name:           req.Name,
			Hostname:       req.Hostname,
			Port:           req.Port,
			Username:       req.Username,
			OSFamily:       "linux",
			ConnectionType: req.ConnectionType,
			Tags:           req.Tags,
			Auth: models.NodeAuth{
				Method:       models.AuthMethod(req.Auth.Method),
				CredentialID: req.Auth.CredentialID,
			},
		}

		if _, err := h.co.CreateNode(c.Request().Context(), node, namespace); err != nil {
			importErrors = append(importErrors, NodeImportError{Row: rowNum, Name: name, Error: err.Error()})
			continue
		}

		imported++
	}

	return c.JSON(http.StatusOK, NodeImportResp{
		Imported: imported,
		Errors:   importErrors,
	})
}
