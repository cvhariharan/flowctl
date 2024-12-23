package handlers

import (
	"net/http"

	"github.com/cvhariharan/autopilot/internal/core"
	"github.com/cvhariharan/autopilot/internal/ui"
	"github.com/cvhariharan/autopilot/internal/ui/partials"
	"github.com/labstack/echo/v4"
	"github.com/spf13/viper"
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

func (h *Handler) HandleDeleteUser(c echo.Context) error {
	userID := c.Param("userID")

	if userID == "" {
		return render(c, partials.InlineError("user id cannot be empty"), http.StatusBadRequest)
	}

	u, err := h.co.GetUserByUUID(c.Request().Context(), userID)
	if err != nil {
		c.Logger().Error(err)
		return render(c, partials.InlineError("could not get user"), http.StatusNotFound)
	}

	// Do not delete admin user
	if u.Username == viper.GetString("app.admin_username") {
		return render(c, partials.InlineError("cannot delete admin user"), http.StatusForbidden)
	}

	err = h.co.DeleteUserByUUID(c.Request().Context(), userID)
	if err != nil {
		c.Logger().Error(err)
		return render(c, partials.InlineError("could not delete user"), http.StatusInternalServerError)
	}

	users, err := h.co.GetAllUsersWithGroups(c.Request().Context())
	if err != nil {
		c.Logger().Error(err)
		return render(c, partials.InlineError("could not get all users"), http.StatusInternalServerError)
	}

	return render(c, ui.UsersTable(users), http.StatusOK)
}

func (h *Handler) HandleCreateUser(c echo.Context) error {
	name := c.FormValue("name")
	username := c.FormValue("username")

	if username == "" || name == "" {
		return render(c, partials.InlineError("name or username cannot be empty"), http.StatusBadRequest)
	}

	_, err := h.co.CreateUser(c.Request().Context(), name, username, core.OIDCLoginType, core.StandardUserRole)
	if err != nil {
		c.Logger().Error(err)
		return render(c, partials.InlineError("could not create user"), http.StatusInternalServerError)
	}

	users, err := h.co.GetAllUsersWithGroups(c.Request().Context())
	if err != nil {
		c.Logger().Error(err)
		render(c, partials.InlineError("could not get all users"), http.StatusInternalServerError)
	}

	return render(c, ui.UsersTable(users), http.StatusOK)
}
