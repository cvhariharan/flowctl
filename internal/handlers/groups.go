package handlers

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
)

func (h *Handler) HandleGetGroup(c echo.Context) error {
	groupID := c.Param("groupID")
	if groupID == "" {
		return wrapError(http.StatusBadRequest, "group id cannot be empty", nil, nil)
	}

	group, err := h.co.GetGroupWithUsers(c.Request().Context(), groupID)
	if err != nil {
		return wrapError(http.StatusNotFound, "could not retrieve group", err, nil)
	}

	return c.JSON(http.StatusOK, GroupWithUsers{
		Group: coreGroupToGroup(group.Group),
		Users: coreUserArrayCast(group.Users),
	})
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

	return c.JSON(http.StatusCreated, GroupWithUsers{
		Group: coreGroupToGroup(group.Group),
		Users: coreUserArrayCast(group.Users),
	})
}

func (h *Handler) HandleUpdateGroup(c echo.Context) error {
	groupID := c.Param("groupID")
	if groupID == "" {
		return wrapError(http.StatusBadRequest, "group id cannot be empty", nil, nil)
	}

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

	group, err := h.co.UpdateGroup(c.Request().Context(), groupID, req.Name, req.Description)
	if err != nil {
		return wrapError(http.StatusBadRequest, "could not update group", err, nil)
	}

	return c.JSON(http.StatusOK, GroupWithUsers{
		Group: coreGroupToGroup(group.Group),
		Users: coreUserArrayCast(group.Users),
	})
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

func (h *Handler) HandleGroupPagination(c echo.Context) error {
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
	g, pageCount, totalCount, err := h.co.SearchGroup(c.Request().Context(), req.Filter, req.Count, req.Count*req.Page)
	if err != nil {
		return wrapError(http.StatusBadRequest, "error retrieving groups", err, nil)
	}

	var groups []GroupWithUsers
	for _, v := range g {
		groups = append(groups, GroupWithUsers{
			Group: coreGroupToGroup(v.Group),
			Users: coreUserArrayCast(v.Users),
		})
	}

	return c.JSON(http.StatusOK, GroupsPaginateResponse{
		Groups:     groups,
		PageCount:  pageCount,
		TotalCount: totalCount,
	})
}
