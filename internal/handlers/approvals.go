package handlers

import (
	"fmt"
	"net/http"

	"github.com/cvhariharan/flowctl/internal/core/models"
	"github.com/labstack/echo/v4"
)

func (h *Handler) HandleApprovalAction(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}

	var req ApprovalActionReq
	if err := c.Bind(&req); err != nil {
		return wrapError(ErrInvalidInput, "invalid request", err, nil)
	}

	if err := h.validate.Struct(req); err != nil {
		return wrapError(ErrValidationFailed, fmt.Sprintf("request validation failed: %s", formatValidationErrors(err)), err, nil)
	}

	user, err := h.getUserInfo(c)
	if err != nil {
		return wrapError(ErrAuthenticationFailed, "could not get user details", err, nil)
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

	err = h.co.ApproveOrRejectAction(c.Request().Context(), req.ApprovalID, user.ID, status, namespace)
	if err != nil {
		return wrapError(ErrOperationFailed, "could not process approval action", err, nil)
	}

	return c.JSON(http.StatusOK, ApprovalActionResp{
		ID:      req.ApprovalID,
		Status:  string(status),
		Message: message,
	})
}

func (h *Handler) HandleGetApproval(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}

	var req ApprovalGetReq
	if err := c.Bind(&req); err != nil {
		return wrapError(ErrInvalidInput, "invalid request", err, nil)
	}

	if err := h.validate.Struct(req); err != nil {
		return wrapError(ErrValidationFailed, fmt.Sprintf("request validation failed: %s", formatValidationErrors(err)), err, nil)
	}

	approval, err := h.co.GetApprovalWithInputs(c.Request().Context(), req.ApprovalID, namespace)
	if err != nil {
		return wrapError(ErrOperationFailed, "could not get approval details", err, nil)
	}

	response := ApprovalDetailsResp{
		ID:          approval.UUID,
		ActionID:    approval.ActionID,
		Status:      string(approval.Status),
		ExecID:      approval.ExecID,
		Inputs:      approval.Inputs,
		DecidedBy:   approval.DecidedBy,
		FlowName:    approval.FlowName,
		FlowID:      approval.FlowID,
		RequestedBy: approval.RequestedBy,
		CreatedAt:   approval.CreatedAt,
		UpdatedAt:   approval.UpdatedAt,
	}

	return c.JSON(http.StatusOK, response)
}

func (h *Handler) HandleListApprovals(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}

	var req ApprovalPaginateRequest
	if err := c.Bind(&req); err != nil {
		return wrapError(ErrInvalidInput, "could not decode request", err, nil)
	}

	if err := h.validate.Struct(req); err != nil {
		return wrapError(ErrValidationFailed, "request validation failed", err, nil)
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

	approvals, pageCount, totalCount, err := h.co.GetApprovalsPaginated(c.Request().Context(), namespace, req.Status, req.Filter, req.Page+1, req.Count)
	if err != nil {
		return wrapError(ErrOperationFailed, "could not get approvals", err, nil)
	}

	approvalResponses := make([]ApprovalResp, len(approvals))
	for i, approval := range approvals {
		approvalResponses[i] = ApprovalResp{
			ID:          approval.UUID,
			ActionID:    approval.ActionID,
			FlowName: 	 approval.FlowName,
			Status:      string(approval.Status),
			ExecID:      approval.ExecID,
			RequestedBy: approval.RequestedBy,
			CreatedAt:   approval.CreatedAt,
			UpdatedAt:   approval.UpdatedAt,
		}
	}

	return c.JSON(http.StatusOK, ApprovalsPaginateResponse{
		Approvals:  approvalResponses,
		PageCount:  pageCount,
		TotalCount: totalCount,
	})
}
