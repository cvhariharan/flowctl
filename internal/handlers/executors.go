package handlers

import (
	"net/http"

	"github.com/cvhariharan/flowctl/sdk/executor"
	"github.com/labstack/echo/v4"
)

func (h *Handler) HandleGetExecutorConfig(c echo.Context) error {
	executorName := c.Param("executor")
	if executorName == "" {
		return wrapError(http.StatusBadRequest, "executor name cannot be empty", nil, nil)
	}

	schema, err := executor.GetSchema(executorName)
	if err != nil {
		return wrapError(http.StatusNotFound, "could not get executor config", err, nil)
	}

	return c.JSON(http.StatusOK, schema)
}
