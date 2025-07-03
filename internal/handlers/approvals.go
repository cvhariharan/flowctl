package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/cvhariharan/autopilot/internal/core/models"
	"github.com/labstack/echo/v4"
)

func (h *Handler) ApprovalMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		approvalID := c.Param("approvalID")
		if approvalID == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "approval ID cannot be empty")
		}

		user, ok := c.Get("user").(models.UserInfo)
		if !ok {
			return echo.NewHTTPError(http.StatusForbidden, "could not get user details")
		}

		areq, err := h.co.GetApprovalRequest(c.Request().Context(), approvalID)
		if err != nil {
			c.Logger().Error(err)
			return echo.NewHTTPError(http.StatusInternalServerError, "could not get approval request")
		}

		c.Logger().Infof("user info: %+v\n", user)

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
					c.Logger().Error(err)
					return echo.NewHTTPError(http.StatusInternalServerError, "could not get group from approvers list")
				}
				for _, group := range user.Groups {
					if group == groupID.ID {
						authorized = true
						break
					}
				}
			}
			if authorized {
				break
			}
		}

		if !authorized {
			return echo.NewHTTPError(http.StatusForbidden, "user is not authorized to perform this action")
		}

		return next(c)
	}
}

func (h *Handler) HandleApprovalRequest(c echo.Context) error {
	approvalID := c.Param("approvalID")
	if approvalID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "approval ID cannot be empty")
	}

	areq, err := h.co.GetApprovalRequest(c.Request().Context(), approvalID)
	if err != nil {
		c.Logger().Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError, "could not get approval request")
	}

	if areq.Status != models.ApprovalStatusPending {
		return wrapError(http.StatusBadRequest, "request has already been processed", nil, nil)
	}

	f, err := h.co.GetFlowFromLogID(areq.ExecID)
	if err != nil {
		c.Logger().Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError, "could not get flow")
	}

	return c.JSON(http.StatusOK, ApprovalRequestResp{
		ID:          approvalID,
		Title:       f.Meta.Name,
		Description: fmt.Sprintf("Approval request to execute action %q from flow %q", areq.ActionID, f.Meta.Name),
		RequestedBy: areq.RequestedBy,
	})
}

func (h *Handler) HandleApprovalAction(c echo.Context) error {
	approvalID := c.Param("approvalID")
	if approvalID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "approval ID cannot be empty")
	}

	action := c.Param("action")
	if action != "approve" && action != "reject" {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid action, must be approve or reject")
	}

	user, ok := c.Get("user").(models.UserInfo)
	if !ok {
		return echo.NewHTTPError(http.StatusForbidden, "could not get user details")
	}

	var status models.ApprovalType
	var message string
	if action == "approve" {
		status = models.ApprovalStatusApproved
		message = "The request has been approved successfully."
	} else {
		status = models.ApprovalStatusRejected
		message = "The request has been rejected."
	}

	err := h.co.ApproveOrRejectAction(c.Request().Context(), approvalID, user.ID, status)
	if err != nil {
		c.Logger().Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError, "could not process approval action")
	}

	return c.JSON(http.StatusOK, ApprovalActionResp{
		ID:      approvalID,
		Status:  string(status),
		Message: message,
	})
}
