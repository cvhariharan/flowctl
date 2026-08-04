package handlers

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
)

// HandleListMyFlowGroups returns flow groups in a namespace that the current user has access to
func (h *Handler) HandleListMyFlowGroups(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}

	user, err := h.getUserInfo(c)
	if err != nil {
		return wrapError(ErrAuthenticationFailed, "could not get user details", err, nil)
	}

	prefixes, err := h.co.GetAccessibleGroups(c.Request().Context(), user.ID, namespace)
	if err != nil {
		return wrapError(ErrOperationFailed, "could not get flow groups", err, nil)
	}

	groups := make([]FlowGroupResp, 0, len(prefixes))
	for _, prefix := range prefixes {
		count, err := h.co.GetFlowCountByPrefix(c.Request().Context(), namespace, prefix.Name)
		if err != nil {
			return wrapError(ErrOperationFailed, "could not get flow count", err, nil)
		}
		groups = append(groups, FlowGroupResp{
			ID:          prefix.ID,
			Prefix:      prefix.Name,
			Description: prefix.Description,
			FlowCount:   count,
		})
	}

	return c.JSON(http.StatusOK, FlowGroupsResponse{Groups: groups})
}

// HandleGetFlowGroup returns all flows in a specific group (prefix)
func (h *Handler) HandleGetFlowGroup(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}

	group := c.Param("group")
	if group == "" {
		return wrapError(ErrRequiredFieldMissing, "group name cannot be empty", nil, nil)
	}

	flows, err := h.co.GetFlowsByPrefix(c.Request().Context(), namespace, group)
	if err != nil {
		return wrapError(ErrOperationFailed, "could not get flows for group", err, nil)
	}

	flowItems := make([]FlowListItem, len(flows))
	for i, flow := range flows {
		flowItems[i] = coreFlowToFlow(flow)
	}

	nextRuns, err := h.co.GetNextScheduledRuns(c.Request().Context(), namespace, flowSlugs(flowItems))
	if err != nil {
		return wrapError(ErrOperationFailed, "could not get next scheduled runs", err, nil)
	}
	attachNextRuns(flowItems, nextRuns)

	return c.JSON(http.StatusOK, FlowListResponse{Flows: flowItems})
}

// HandleListFlowGroups returns all flow groups in a namespace with flow counts
func (h *Handler) HandleListFlowGroups(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}

	prefixes, err := h.co.ListFlowPrefixes(c.Request().Context(), namespace)
	if err != nil {
		return wrapError(ErrOperationFailed, "could not list flow groups", err, nil)
	}

	groups := make([]FlowGroupResp, 0, len(prefixes))
	for _, p := range prefixes {
		count, err := h.co.GetFlowCountByPrefix(c.Request().Context(), namespace, p.Name)
		if err != nil {
			return wrapError(ErrOperationFailed, "could not get flow count", err, nil)
		}
		groups = append(groups, FlowGroupResp{
			ID:          p.ID,
			Prefix:      p.Name,
			Description: p.Description,
			FlowCount:   count,
		})
	}

	return c.JSON(http.StatusOK, FlowGroupsResponse{Groups: groups})
}

// HandleCreateFlowGroup creates a new flow group in a namespace
func (h *Handler) HandleCreateFlowGroup(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}

	var req FlowGroupReq
	if err := c.Bind(&req); err != nil {
		return wrapError(ErrInvalidInput, "invalid request", err, nil)
	}

	if err := h.validate.Struct(req); err != nil {
		return wrapError(ErrValidationFailed, fmt.Sprintf("request validation failed: %s", formatValidationErrors(err)), err, nil)
	}

	prefix, err := h.co.CreateFlowPrefix(c.Request().Context(), namespace, req.Name, req.Description)
	if err != nil {
		return wrapError(ErrOperationFailed, "could not create flow group", err, nil)
	}

	return c.JSON(http.StatusCreated, FlowGroupDetailResp{
		ID:          prefix.ID,
		Name:        prefix.Name,
		Description: prefix.Description,
	})
}

// HandleUpdateFlowGroup updates an existing flow group
func (h *Handler) HandleUpdateFlowGroup(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}

	groupID := c.Param("groupID")
	if groupID == "" {
		return wrapError(ErrRequiredFieldMissing, "group ID is required", nil, nil)
	}

	var req FlowGroupReq
	if err := c.Bind(&req); err != nil {
		return wrapError(ErrInvalidInput, "invalid request", err, nil)
	}

	if err := h.validate.Struct(req); err != nil {
		return wrapError(ErrValidationFailed, fmt.Sprintf("request validation failed: %s", formatValidationErrors(err)), err, nil)
	}

	prefix, err := h.co.UpdateFlowPrefix(c.Request().Context(), groupID, namespace, req.Name, req.Description)
	if err != nil {
		return wrapError(ErrOperationFailed, "could not update flow group", err, nil)
	}

	return c.JSON(http.StatusOK, FlowGroupDetailResp{
		ID:          prefix.ID,
		Name:        prefix.Name,
		Description: prefix.Description,
	})
}

// HandleDeleteFlowGroup deletes a flow group
func (h *Handler) HandleDeleteFlowGroup(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}

	groupID := c.Param("groupID")
	if groupID == "" {
		return wrapError(ErrRequiredFieldMissing, "group ID is required", nil, nil)
	}

	if err := h.co.DeleteFlowPrefix(c.Request().Context(), groupID, namespace); err != nil {
		return wrapError(ErrOperationFailed, "could not delete flow group", err, nil)
	}

	return c.NoContent(http.StatusOK)
}
