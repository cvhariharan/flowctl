package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/cvhariharan/flowctl/internal/core"
	"github.com/labstack/echo/v4"
)

func parseVersion(value string) (int, error) {
	n, err := strconv.ParseInt(value, 10, 32)
	if err == nil && n < 0 {
		err = errors.New("version must be non-negative")
	}
	if err != nil {
		return 0, wrapError(ErrInvalidInput, "version must be a non-negative integer", err, nil)
	}
	return int(n), nil
}

func flowVersionError(err error) error {
	switch {
	case errors.Is(err, core.ErrFlowNotFound), errors.Is(err, core.ErrFlowVersionNotFound):
		return wrapError(ErrResourceNotFound, err.Error(), err, nil)
	case errors.Is(err, core.ErrFlowVersionFormatMismatch):
		return wrapError(ErrResourceConflict, err.Error(), err, nil)
	case errors.Is(err, core.ErrFlowVersionInvalid):
		he := wrapError(ErrValidationFailed, err.Error(), err, nil).(*HTTPError)
		he.code = http.StatusUnprocessableEntity
		return he
	default:
		return wrapError(ErrOperationFailed, "could not load flow version", err, nil)
	}
}

func (h *Handler) HandleListFlowVersions(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}
	var req struct {
		Page  int `query:"page"`
		Count int `query:"count"`
	}
	if err := c.Bind(&req); err != nil {
		return wrapError(ErrInvalidInput, "could not decode request", err, nil)
	}
	if req.Page < 0 || req.Count < 0 || req.Count > 1<<31-1 {
		return wrapError(ErrInvalidPagination, "invalid pagination parameters", nil, nil)
	}
	if req.Count == 0 {
		req.Count = CountPerPage
	}
	if req.Page > 0 {
		req.Page--
	}
	if req.Page > (1<<31-1)/req.Count {
		return wrapError(ErrInvalidPagination, "invalid pagination parameters", nil, nil)
	}
	versions, pageCount, totalCount, err := h.co.ListFlowVersions(c.Request().Context(), c.Param("flowID"), namespace, req.Count, req.Count*req.Page)
	if err != nil {
		return flowVersionError(err)
	}
	items := make([]FlowVersionListItem, 0, len(versions))
	for _, v := range versions {
		items = append(items, FlowVersionListItem{Version: v.Version, Checksum: v.Checksum, CreatedAt: v.CreatedAt.Format(TimeFormat), Active: v.Active})
	}
	return c.JSON(http.StatusOK, FlowVersionsResp{Versions: items, PageCount: pageCount, TotalCount: totalCount})
}

func (h *Handler) HandleGetFlowVersion(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}
	version, err := parseVersion(c.Param("version"))
	if err != nil {
		return err
	}
	v, err := h.co.GetFlowVersionSource(c.Request().Context(), c.Param("flowID"), namespace, version)
	if err != nil {
		return flowVersionError(err)
	}
	return c.JSON(http.StatusOK, FlowVersionResp{Version: v.Version, Format: v.Format, Source: v.Source, CreatedAt: v.CreatedAt.Format(TimeFormat), Active: v.Active})
}

func (h *Handler) HandleActivateFlowVersion(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}
	version, err := parseVersion(c.Param("version"))
	if err != nil {
		return err
	}
	f, err := h.co.ActivateFlowVersion(c.Request().Context(), c.Param("flowID"), namespace, version)
	if err != nil {
		return flowVersionError(err)
	}
	return c.JSON(http.StatusOK, FlowVersionActivateResp{ID: f.Meta.ID, Version: f.Meta.Version})
}

func (h *Handler) HandleDiffFlowVersions(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}
	from, err := parseVersion(c.QueryParam("from"))
	if err != nil {
		return err
	}
	to := 0
	if value := c.QueryParam("to"); value != "" {
		to, err = parseVersion(value)
		if err != nil {
			return err
		}
	}
	diff, err := h.co.DiffFlowVersions(c.Request().Context(), c.Param("flowID"), namespace, from, to)
	if err != nil {
		return flowVersionError(err)
	}
	return c.JSON(http.StatusOK, diff)
}
