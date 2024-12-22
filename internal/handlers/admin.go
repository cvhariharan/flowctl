package handlers

import (
	"net/http"

	"github.com/cvhariharan/autopilot/internal/ui"
	"github.com/labstack/echo/v4"
)

func (h *Handler) HandleGroup(c echo.Context) error {
	if c.QueryParam("action") == "add" {
		return render(c, ui.GroupModal())
	}

	groups, err := h.co.GetAllGroups(c.Request().Context())
	if err != nil {
		c.Logger().Error(err)
		return render(c, ui.GroupManagementPage(groups))
	}

	return render(c, ui.GroupManagementPage(groups))
}

func (h *Handler) HandleCreateGroup(c echo.Context) error {
	groupName := c.FormValue("name")
	// groupDescription := c.FormValue("description")

	if groupName == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "name cannot be empty")
	}

	return c.NoContent(http.StatusCreated)
}

func (h *Handler) HandleDeleteGroup(c echo.Context) error {
	groupID := c.Param("groupID")

	if groupID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "group id cannot be empty")
	}

	_, err := h.co.GetGroupByUUID(c.Request().Context(), groupID)
	if err != nil {
		c.Logger().Error(err)
		return echo.NewHTTPError(http.StatusNotFound, "group not found")
	}

	if err := h.co.DeleteGroupByUUID(c.Request().Context(), groupID); err != nil {
		c.Logger().Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError, "could not delete group")
	}

	groups, err := h.co.GetAllGroups(c.Request().Context())
	if err != nil {
		c.Logger().Error(err)
		return render(c, ui.GroupsTable(groups))
	}

	return render(c, ui.GroupsTable(groups))
}
