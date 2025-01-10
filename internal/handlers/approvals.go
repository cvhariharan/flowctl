package handlers

import (
	"net/http"

	"github.com/cvhariharan/autopilot/internal/models"
	"github.com/labstack/echo/v4"
)

func (h *Handler) HandleApprovalRequest(c echo.Context) error {
	execID := c.Param("execID")

	f, _ := h.co.GetFlowFromLogID(execID)

	r, err := h.co.RequestApproval(c.Request().Context(), execID, f.Actions[0])
	if err != nil {
		c.Logger().Error(err)
		return err
	}

	return c.String(http.StatusOK, r)
}

func (h *Handler) HandleApprovalAction(c echo.Context) error {
	approvalID := c.Param("approvalID")

	if approvalID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "approval ID cannot be empty")
	}

	if err := h.co.ApproveOrRejectAction(c.Request().Context(), approvalID, models.ApprovalStatusApproved); err != nil {
		c.Logger().Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError, "could not approve request")
	}

	return c.NoContent(http.StatusOK)
}
