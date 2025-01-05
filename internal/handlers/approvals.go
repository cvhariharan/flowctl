package handlers

import (
	"net/http"

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
