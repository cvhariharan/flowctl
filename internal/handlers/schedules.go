package handlers

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/cvhariharan/flowctl/internal/core/models"
	"github.com/labstack/echo/v4"
)

func scheduleInputError(err error) error {
	var verr *models.FlowValidationError
	if !errors.As(err, &verr) {
		return nil
	}
	return wrapError(ErrValidationFailed, "", err, FlowInputValidationError{
		FieldName:  verr.FieldName,
		ErrMessage: verr.Msg,
	})
}

func (h *Handler) HandleCreateSchedule(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}

	user, err := h.getUserInfo(c)
	if err != nil {
		return wrapError(ErrAuthenticationFailed, "could not get user details", err, nil)
	}

	var req ScheduleCreateReq
	if err := c.Bind(&req); err != nil {
		return wrapError(ErrInvalidInput, "could not decode request", err, nil)
	}

	if err := h.validate.Struct(req); err != nil {
		return wrapError(ErrValidationFailed, fmt.Sprintf("request validation failed: %s", formatValidationErrors(err)), err, nil)
	}

	schedule, err := h.co.CreateSchedule(c.Request().Context(), req.FlowID, req.Cron, req.Timezone, req.Inputs, user.ID, namespace)
	if err != nil {
		if valErr := scheduleInputError(err); valErr != nil {
			return valErr
		}
		return wrapError(ErrOperationFailed, err.Error(), err, nil)
	}

	return c.JSON(http.StatusCreated, coreScheduleToScheduleResp(schedule))
}

func (h *Handler) HandleGetSchedule(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}

	user, err := h.getUserInfo(c)
	if err != nil {
		return wrapError(ErrAuthenticationFailed, "could not get user details", err, nil)
	}

	req := ScheduleGetReq{
		FlowID:     c.Param("flowID"),
		ScheduleID: c.Param("schedule_id"),
	}
	if err := h.validate.Struct(req); err != nil {
		return wrapError(ErrValidationFailed, fmt.Sprintf("request validation failed: %s", formatValidationErrors(err)), err, nil)
	}

	schedule, err := h.co.GetSchedule(c.Request().Context(), req.ScheduleID, user.ID, namespace)
	if err != nil {
		return wrapError(ErrResourceNotFound, "schedule not found", err, nil)
	}

	return c.JSON(http.StatusOK, coreScheduleToScheduleResp(schedule))
}

func (h *Handler) HandleListSchedules(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}

	user, err := h.getUserInfo(c)
	if err != nil {
		return wrapError(ErrAuthenticationFailed, "could not get user details", err, nil)
	}

	var req ScheduleListReq
	if err := c.Bind(&req); err != nil {
		return wrapError(ErrInvalidInput, "could not decode request", err, nil)
	}

	if err := h.validate.Struct(req); err != nil {
		return wrapError(ErrValidationFailed, fmt.Sprintf("request validation failed: %s", formatValidationErrors(err)), err, nil)
	}

	if req.Page < 0 || req.Count < 0 {
		return wrapError(ErrInvalidPagination, "invalid pagination parameters", nil, nil)
	}

	if req.Page > 0 {
		req.Page -= 1
	}

	if req.Count == 0 {
		req.Count = CountPerPage
	}

	schedules, pageCount, totalCount, err := h.co.ListSchedules(c.Request().Context(), req.FlowID, user.ID, namespace, req.Count, req.Count*req.Page)
	if err != nil {
		return wrapError(ErrOperationFailed, "could not list schedules", err, nil)
	}

	return c.JSON(http.StatusOK, SchedulesPaginateResponse{
		Schedules:  coreSchedulesToScheduleResps(schedules),
		PageCount:  pageCount,
		TotalCount: totalCount,
	})
}

func (h *Handler) HandleUpdateSchedule(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}

	user, err := h.getUserInfo(c)
	if err != nil {
		return wrapError(ErrAuthenticationFailed, "could not get user details", err, nil)
	}

	var req ScheduleUpdateReq
	if err := c.Bind(&req); err != nil {
		return wrapError(ErrInvalidInput, "could not decode request", err, nil)
	}

	if err := h.validate.Struct(req); err != nil {
		return wrapError(ErrValidationFailed, fmt.Sprintf("request validation failed: %s", formatValidationErrors(err)), err, nil)
	}

	schedule, err := h.co.UpdateSchedule(c.Request().Context(), req.ScheduleID, req.Cron, req.Timezone, req.Inputs, req.IsActive, user.ID, namespace)
	if err != nil {
		if valErr := scheduleInputError(err); valErr != nil {
			return valErr
		}
		return wrapError(ErrOperationFailed, err.Error(), err, nil)
	}

	return c.JSON(http.StatusOK, ScheduleUpdateResp{
		ScheduleID: schedule.UUID,
	})
}

func (h *Handler) HandleDeleteSchedule(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}

	user, err := h.getUserInfo(c)
	if err != nil {
		return wrapError(ErrAuthenticationFailed, "could not get user details", err, nil)
	}

	req := ScheduleGetReq{
		FlowID:     c.Param("flowID"),
		ScheduleID: c.Param("schedule_id"),
	}
	if err := h.validate.Struct(req); err != nil {
		return wrapError(ErrValidationFailed, fmt.Sprintf("request validation failed: %s", formatValidationErrors(err)), err, nil)
	}

	err = h.co.DeleteSchedule(c.Request().Context(), req.ScheduleID, user.ID, namespace)
	if err != nil {
		return wrapError(ErrOperationFailed, "could not delete schedule", err, nil)
	}

	return c.NoContent(http.StatusOK)
}
