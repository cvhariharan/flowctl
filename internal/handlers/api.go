package handlers

import (
	"fmt"
	"net/http"

	"github.com/cvhariharan/autopilot/internal/models"
	"github.com/expr-lang/expr"
	"github.com/labstack/echo/v4"
)

type Handler struct {
	flows map[string]models.Flow
}

func NewHandler(f map[string]models.Flow) *Handler {
	return &Handler{flows: f}
}

// HandleTrigger responds to API calls with an input.
// Input is of the form name=>value
func (h *Handler) HandleTrigger(c echo.Context) error {
	var req map[string]interface{}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error validating request bind")
	}

	flowName := c.Param("flow")
	flow, ok := h.flows[flowName]
	if !ok {
		return echo.NewHTTPError(http.StatusNotFound, "requested flow not found")
	}

	var inputReq map[string]interface{}
	for _, input := range flow.Inputs {
		val, ok := req[input.Name]
		if !ok && input.Required && input.Default == "" {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("required field %s missing", input.Name))
		}
		valStr := fmt.Sprintf("%v", val)

		if valStr == "" {
			valStr = input.Default
		}

		// expr validation
		env := map[string]interface{}{
			input.Name: val,
		}

		program, err := expr.Compile(input.Validation, expr.Env(env))
		if err != nil {
			return err
		}

		output, err := expr.Run(program, env)
		if err != nil {
			return err
		}

		isValid, ok := output.(bool)
		if !ok {
			return fmt.Errorf("error validating request: expected boolean response")
		}

		if !isValid {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("%s failed validation", input.Name))
		}

		// input val is valid
		inputReq[input.Name] = val
	}

	return c.JSON(http.StatusOK, inputReq)
}
