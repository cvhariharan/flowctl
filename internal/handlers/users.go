package handlers

import (
	"fmt"
	"net/http"

	"github.com/cvhariharan/autopilot/internal/core/models"
	"github.com/labstack/echo/v4"
	"github.com/spf13/viper"
)

func (h *Handler) HandleGetUserProfile(c echo.Context) error {
	user, err := h.getUserInfo(c)
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not retrieve user info", err, nil)
	}

	return c.JSON(http.StatusOK, coreUserInfoToUserProfile(user))
}

func (h *Handler) HandleGetUser(c echo.Context) error {
	userID := c.Param("userID")
	if userID == "" {
		return wrapError(http.StatusBadRequest, "user ID cannot be empty", nil, nil)
	}

	u, err := h.co.GetUserWithUUIDWithGroups(c.Request().Context(), userID)
	if err != nil {
		return wrapError(http.StatusNotFound, "user not found", err, nil)
	}

	return c.JSON(http.StatusOK, UserWithGroups{
		User:   coreUsertoUser(u.User),
		Groups: coreGroupArrayCast(u.Groups),
	})
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
		Groups   []string `json:"groups"`
	}
	if err := c.Bind(&req); err != nil {
		return wrapError(http.StatusBadRequest, "could not decode request", err, nil)
	}

	h.logger.Debug("update user request", "userID", userID, "name", req.Name, "username", req.Username, "groups", req.Groups)

	if err := h.validate.Struct(req); err != nil {
		return wrapError(http.StatusBadRequest, fmt.Sprintf("request validation failed: %s", formatValidationErrors(err)), err, nil)
	}

	if req.Name == "" || req.Username == "" {
		return wrapError(http.StatusBadRequest, "name and username cannot be empty", nil, nil)
	}

	user, err := h.co.UpdateUser(c.Request().Context(), userID, req.Name, req.Username, req.Groups)
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not update user", err, nil)
	}

	return c.JSON(http.StatusOK, UserWithGroups{
		User:   coreUsertoUser(user.User),
		Groups: coreGroupArrayCast(user.Groups),
	})
}

func (h *Handler) HandleUserPagination(c echo.Context) error {
	var req PaginateRequest
	if err := c.Bind(&req); err != nil {
		return wrapError(http.StatusInternalServerError, "invalid request", err, nil)
	}

	if req.Page < 0 || req.Count < 0 {
		return wrapError(http.StatusInternalServerError, "invalid request, page or count per page cannot be less than 0", fmt.Errorf("page and count per page less than zero"), nil)
	}

	if req.Page > 0 {
		req.Page -= 1
	}

	if req.Count == 0 {
		req.Count = CountPerPage
	}
	h.logger.Debug("user pagination", "filter", req.Filter)
	u, pageCount, totalCount, err := h.co.SearchUser(c.Request().Context(), req.Filter, req.Count, req.Count*req.Page)
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not search for users", err, nil)
	}

	var users []UserWithGroups
	for _, v := range u {
		users = append(users, UserWithGroups{
			User:   coreUsertoUser(v.User),
			Groups: coreGroupArrayCast(v.Groups),
		})
	}

	return c.JSON(http.StatusOK, UsersPaginateResponse{
		Users:      users,
		PageCount:  pageCount,
		TotalCount: totalCount,
	})
}

func (h *Handler) HandleDeleteUser(c echo.Context) error {
	userID := c.Param("userID")

	if userID == "" {
		return wrapError(http.StatusBadRequest, "user id cannot be empty", nil, nil)
	}

	u, err := h.co.GetUserByUUID(c.Request().Context(), userID)
	if err != nil {
		return wrapError(http.StatusNotFound, "could not retrieve user", err, nil)
	}

	// Do not delete admin user
	if u.Username == viper.GetString("app.admin_username") {
		return wrapError(http.StatusForbidden, "cannot delete admin user", nil, nil)
	}

	err = h.co.DeleteUserByUUID(c.Request().Context(), userID)
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not delete user", err, nil)
	}

	return c.NoContent(http.StatusOK)
}

func (h *Handler) HandleCreateUser(c echo.Context) error {
	var req struct {
		Name     string `json:"name" validate:"required,min=4,max=30,alphanum_whitespace"`
		Username string `json:"username" validate:"required,email"`
	}
	if err := c.Bind(&req); err != nil {
		return wrapError(http.StatusBadRequest, "could not decode request", err, nil)
	}

	if err := h.validate.Struct(req); err != nil {
		return wrapError(http.StatusBadRequest, fmt.Sprintf("request validation failed: %s", formatValidationErrors(err)), err, nil)
	}

	u, err := h.co.CreateUser(c.Request().Context(), req.Name, req.Username, models.OIDCLoginType, models.StandardUserRole)
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not create user", err, nil)
	}

	user, err := h.co.GetUserWithUUIDWithGroups(c.Request().Context(), u.ID)
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not retrieve created user", err, nil)
	}

	return c.JSON(http.StatusCreated, UserWithGroups{
		User:   coreUsertoUser(user.User),
		Groups: coreGroupArrayCast(user.Groups),
	})
}
