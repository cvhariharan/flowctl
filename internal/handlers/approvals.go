package handlers

import (
	"net/http"
	"strings"

	"github.com/cvhariharan/autopilot/internal/core/models"
	"github.com/labstack/echo/v4"
	"slices"
)

func (h *Handler) ApprovalMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		approvalID := c.Param("approvalID")
		if approvalID == "" {
			return wrapError(http.StatusBadRequest, "approval ID cannot be empty", nil, nil)
		}

		namespace, ok := c.Get("namespace").(string)
		if !ok {
			return wrapError(http.StatusBadRequest, "could not get namespace", nil, nil)
		}

		user, ok := c.Get("user").(models.UserInfo)
		if !ok {
			return wrapError(http.StatusForbidden, "could not get user details", nil, nil)
		}

		areq, err := h.co.GetApprovalRequest(c.Request().Context(), approvalID, namespace)
		if err != nil {
			return wrapError(http.StatusInternalServerError, "could not get approval request", err, nil)
		}

		// Check if user is in approvers list
		var authorized bool
		for _, approver := range areq.Approvers {
			switch {
			case strings.HasPrefix(approver, "users/"):
				username := strings.TrimPrefix(approver, "users/")
				if username == user.Username {
					authorized = true
				}
			case strings.HasPrefix(approver, "groups/"):
				groupName := strings.TrimPrefix(approver, "groups/")
				groupID, err := h.co.GetGroupByName(c.Request().Context(), groupName)
				if err != nil {
					return wrapError(http.StatusInternalServerError, "could not get group from approvers list", err, nil)
				}
				if slices.Contains(user.Groups, groupID.ID) {
					authorized = true
				}
			}
			if authorized {
				break
			}
		}

		if !authorized {
			return wrapError(http.StatusForbidden, "user is not authorized to perform this action", nil, nil)
		}

		return next(c)
	}
}

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
