package handlers

import (
	"fmt"
	"net/http"

	"github.com/cvhariharan/autopilot/internal/models"
	"github.com/cvhariharan/autopilot/internal/ui"
	"github.com/cvhariharan/autopilot/internal/ui/partials"
	"github.com/labstack/echo/v4"
	"github.com/spf13/viper"
)

func (h *Handler) HandleUser(c echo.Context) error {
	users, err := h.co.GetAllUsersWithGroups(c.Request().Context())
	if err != nil {
		return wrapError(http.StatusBadRequest, "could not get users", err, nil)
	}

	return c.JSON(http.StatusOK, users)
}

func (h *Handler) HandleUpdateUser(c echo.Context) error {
	userID := c.Param("userID")
	if userID == "" {
		return wrapError(http.StatusBadRequest, "user ID cannot be empty", nil, nil)
	}

	_, err := h.co.GetUserWithUUIDWithGroups(c.Request().Context(), userID)
	if err != nil {
		return wrapError(http.StatusNotFound, "user not found", err, nil)
	}

	var req struct {
		Name     string   `json:"name" validate:"required,min=4,max=30,alphanum_whitespace"`
		Username string   `json:"username" validate:"required,email"`
		Groups   []string `json:"groups[]"`
	}
	if err := c.Bind(&req); err != nil {
		return wrapError(http.StatusBadRequest, "could not decode request", err, nil)
	}

	if err := h.validate.Struct(req); err != nil {
		return wrapError(http.StatusBadRequest, fmt.Sprintf("request validation failed: %s", formatValidationErrors(err)), err, nil)
	}

	if req.Name == "" || req.Username == "" {
		return wrapError(http.StatusBadRequest, "name and username cannot be empty", nil, nil)
	}

	_, err = h.co.UpdateUser(c.Request().Context(), userID, req.Name, req.Username, req.Groups)
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not update user", err, nil)
	}

	users, err := h.co.GetAllUsersWithGroups(c.Request().Context())
	if err != nil {
		return wrapError(http.StatusBadRequest, "could not retrieve users", err, nil)
	}

	return c.JSON(http.StatusOK, users)
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
	var req struct {
		Name     string `form:"name" validate:"required,min=4,max=30,alphanum_whitespace"`
		Username string `form:"username" validate:"required,email"`
	}
	if err := c.Bind(&req); err != nil {
		return render(c, partials.InlineError("could not decode request"), http.StatusBadRequest)
	}

	if err := h.validate.Struct(req); err != nil {
		c.Logger().Error(err)
		return render(c, partials.InlineError(fmt.Sprintf("request validation failed: %s", formatValidationErrors(err))), http.StatusBadRequest)
	}

	_, err := h.co.CreateUser(c.Request().Context(), req.Name, req.Username, models.OIDCLoginType, models.StandardUserRole)
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
