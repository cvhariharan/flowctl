package handlers

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
)

func (h *Handler) HandleGroup(c echo.Context) error {
	groups, err := h.co.GetAllGroupsWithUsers(c.Request().Context())
	if err != nil {
		return wrapError(http.StatusNotFound, "could not get groups", err, nil)
	}

	return c.JSON(http.StatusOK, groups)
}

func (h *Handler) HandleCreateGroup(c echo.Context) error {
	var req struct {
		Name        string `form:"name" validate:"required,alphanum_underscore,min=4,max=30"`
		Description string `form:"description" validate:"max=150"`
	}
	if err := c.Bind(&req); err != nil {
		return wrapError(http.StatusBadRequest, "could not decode request", err, nil)
	}

	if err := h.validate.Struct(req); err != nil {
		return wrapError(http.StatusBadRequest, fmt.Sprintf("request validation failed: %s", formatValidationErrors(err)), err, nil)
	}

	group, err := h.co.CreateGroup(c.Request().Context(), req.Name, req.Description)
	if err != nil {
		return wrapError(http.StatusBadRequest, "could not create group", err, nil)
	}

	return c.JSON(http.StatusCreated, coreGroupToGroup(group))
}

func (h *Handler) HandleDeleteGroup(c echo.Context) error {
	groupID := c.Param("groupID")

	if groupID == "" {
		return wrapError(http.StatusBadRequest, "group id cannot be empty", nil, nil)
	}

	_, err := h.co.GetGroupByUUID(c.Request().Context(), groupID)
	if err != nil {
		return wrapError(http.StatusBadRequest, "could not get group", err, nil)
	}

	if err := h.co.DeleteGroupByUUID(c.Request().Context(), groupID); err != nil {
		return wrapError(http.StatusBadRequest, "could not delete group", err, nil)
	}

	return c.NoContent(http.StatusOK)
}

func (h *Handler) HandleGroupSearch(c echo.Context) error {
	g, err := h.co.SearchGroup(c.Request().Context(), c.QueryParam("search"))
	if err != nil {
		return wrapError(http.StatusBadRequest, "error retrieving groups", err, nil)
	}

	var groups []Group
	for _, v := range g {
		groups = append(groups, coreGroupToGroup(v))
	}

	return c.JSON(http.StatusOK, groups)
}
