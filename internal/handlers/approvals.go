package handlers

import (
	"net/http"

	"github.com/cvhariharan/autopilot/internal/core/models"
	"github.com/labstack/echo/v4"
)


func (h *Handler) HandleApprovalAction(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(http.StatusBadRequest, "could not get namespace", nil, nil)
	}

	approvalID := c.Param("approvalID")
	if approvalID == "" {
		return wrapError(http.StatusBadRequest, "approval ID cannot be empty", nil, nil)
	}

	var req ApprovalActionReq
	if err := c.Bind(&req); err != nil {
		return wrapError(http.StatusBadRequest, "invalid request", err, nil)
	}

	if req.Action != "approve" && req.Action != "reject" {
		return wrapError(http.StatusBadRequest, "invalid action, must be approve or reject", nil, nil)
	}

	user, ok := c.Get("user").(models.UserInfo)
	if !ok {
		return wrapError(http.StatusForbidden, "could not get user details", nil, nil)
	}

	var status models.ApprovalType
	var message string
	if req.Action == "approve" {
		status = models.ApprovalStatusApproved
		message = "The request has been approved successfully."
	} else {
		status = models.ApprovalStatusRejected
		message = "The request has been rejected."
	}

	err := h.co.ApproveOrRejectAction(c.Request().Context(), approvalID, user.ID, status, namespace)
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not process approval action", err, nil)
	}

	return c.JSON(http.StatusOK, ApprovalActionResp{
		ID:      approvalID,
		Status:  string(status),
		Message: message,
	})
}

func (h *Handler) HandleListApprovals(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(http.StatusBadRequest, "could not get namespace", nil, nil)
	}

	var req ApprovalPaginateRequest
	if err := c.Bind(&req); err != nil {
		return wrapError(http.StatusBadRequest, "could not decode request", err, nil)
	}

	if err := h.validate.Struct(req); err != nil {
		return wrapError(http.StatusBadRequest, "request validation failed", err, nil)
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

	approvals, pageCount, totalCount, err := h.co.GetApprovalsPaginated(c.Request().Context(), namespace, req.Status, req.Filter, req.Page+1, req.Count)
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not get approvals", err, nil)
	}

	approvalResponses := make([]ApprovalResp, len(approvals))
	for i, approval := range approvals {
		approvalResponses[i] = ApprovalResp{
			ID:          approval.Uuid.String(),
			ActionID:    approval.ActionID,
			Status:      string(approval.Status),
			ExecID:      approval.ExecID,
			RequestedBy: approval.RequestedBy,
			CreatedAt:   approval.CreatedAt.Format(TimeFormat),
			UpdatedAt:   approval.UpdatedAt.Format(TimeFormat),
		}
	}

	return c.JSON(http.StatusOK, ApprovalsPaginateResponse{
		Approvals:  approvalResponses,
		PageCount:  pageCount,
		TotalCount: totalCount,
	})
}
