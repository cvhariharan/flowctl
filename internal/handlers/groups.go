package handlers

import (
	"net/http"

	"github.com/cvhariharan/autopilot/internal/ui"
	"github.com/cvhariharan/autopilot/internal/ui/partials"
	"github.com/labstack/echo/v4"
)

func (h *Handler) HandleGroup(c echo.Context) error {
	if c.QueryParam("action") == "add" {
		return render(c, ui.GroupModal(), http.StatusOK)
	}

	groups, err := h.co.GetAllGroupsWithUsers(c.Request().Context())
	if err != nil {
		c.Logger().Error(err)
		return render(c, partials.InlineError(err.Error()), http.StatusInternalServerError)
	}

	return render(c, ui.GroupManagementPage(groups, ""), http.StatusOK)
}

func (h *Handler) HandleCreateGroup(c echo.Context) error {
	groupName := c.FormValue("name")
	groupDescription := c.FormValue("description")

	if groupName == "" {
		return render(c, partials.InlineError("name cannot be empty"), http.StatusBadRequest)
	}

	_, err := h.co.CreateGroup(c.Request().Context(), groupName, groupDescription)
	if err != nil {
		c.Logger().Error(err)
		return render(c, partials.InlineError("could not create group"), http.StatusInternalServerError)
	}

	groups, err := h.co.GetAllGroupsWithUsers(c.Request().Context())
	if err != nil {
		c.Logger().Error(err)
		return render(c, partials.InlineError("could not get groups"), http.StatusInternalServerError)
	}

	return render(c, ui.GroupsTable(groups), http.StatusOK)
}

func (h *Handler) HandleDeleteGroup(c echo.Context) error {
	groupID := c.Param("groupID")

	if groupID == "" {
		return render(c, partials.InlineError("group id cannot be empty"), http.StatusBadRequest)
	}

	_, err := h.co.GetGroupByUUID(c.Request().Context(), groupID)
	if err != nil {
		c.Logger().Error(err)
		return render(c, partials.InlineError("could not get group"), http.StatusNotFound)
	}

	if err := h.co.DeleteGroupByUUID(c.Request().Context(), groupID); err != nil {
		c.Logger().Error(err)
		return render(c, partials.InlineError("could not delete group"), http.StatusInternalServerError)
	}

	groups, err := h.co.GetAllGroupsWithUsers(c.Request().Context())
	if err != nil {
		c.Logger().Error(err)
		return render(c, partials.InlineError("could not get groups"), http.StatusInternalServerError)
	}

	return render(c, ui.GroupsTable(groups), http.StatusOK)
}

func (h *Handler) HandleGroupSearch(c echo.Context) error {
	g, err := h.co.SearchGroup(c.Request().Context(), c.QueryParam("search"))
	if err != nil {
		return render(c, partials.InlineError("could not search for groups"), http.StatusInternalServerError)
	}

	return render(c, ui.GroupsTable(g), http.StatusOK)
}
