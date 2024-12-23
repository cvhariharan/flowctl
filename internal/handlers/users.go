package handlers

import (
	"net/http"

	"github.com/cvhariharan/autopilot/internal/ui"
	"github.com/cvhariharan/autopilot/internal/ui/partials"
	"github.com/labstack/echo/v4"
)

func (h *Handler) HandleUser(c echo.Context) error {
	if c.QueryParam("action") == "add" {
		return render(c, ui.UserModal(), http.StatusOK)
	}

	users, err := h.co.GetAllUsersWithGroups(c.Request().Context())
	if err != nil {
		c.Logger().Error(err)
		return render(c, partials.InlineError("could not get all users"), http.StatusInternalServerError)
	}

	return render(c, ui.UserManagementPage(users, ""), http.StatusOK)
}

func (h *Handler) HandleUserSearch(c echo.Context) error {
	u, err := h.co.SearchUser(c.Request().Context(), c.QueryParam("search"))
	if err != nil {
		return render(c, partials.InlineError("could not search for users"), http.StatusInternalServerError)
	}

	return render(c, ui.UsersTable(u), http.StatusOK)
}
