package handlers

import (
	"encoding/json"
	"html/template"
	"io"
	"net/http"

	"github.com/cvhariharan/autopilot/internal/core/models"
	"github.com/labstack/echo/v4"
)

type Template struct {
	Templates *template.Template
}

func NewTemplateRenderer(templateGlob string) Template {
	funcMap := template.FuncMap{
		"toJson": func(v interface{}) string {
			jsonBytes, err := json.Marshal(v)
			if err != nil {
				return "{}"
			}
			return string(jsonBytes)
		},
	}

	tmpl := template.New("").Funcs(funcMap)
	return Template{
		Templates: template.Must(tmpl.ParseGlob("web/**/**/*.html")),
	}
}

func (t Template) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.Templates.ExecuteTemplate(w, name, data)
}

type Page struct {
	Title      string
	ErrMessage string
	Message    string
}

func (h *Handler) HandleLoginView(c echo.Context) error {
	return c.Render(http.StatusOK, "login", nil)
}

func (h *Handler) HandleFlowsListView(c echo.Context) error {
	data := struct {
		Page
		Flows []models.Flow
	}{
		Page: Page{
			Title: "Flows",
		},
	}

	flows, err := h.co.GetAllFlows()
	if err != nil {
		data.ErrMessage = err.Error()
		return c.Render(http.StatusBadRequest, "flows", data)
	}

	data.Flows = flows
	return c.Render(http.StatusOK, "flows", data)
}

func (h *Handler) HandleFlowFormView(c echo.Context) error {
	data := struct {
		Page
		Flow        models.Flow
		InputErrors map[string]string
	}{
		Page: Page{
			Title: "Flow Input",
		},
	}
	flow, err := h.co.GetFlowByID(c.Param("flow"))
	if err != nil {
		data.ErrMessage = err.Error()
		return c.Render(http.StatusBadRequest, "flow_input", data)
	}
	data.Flow = flow

	return c.Render(http.StatusOK, "flow_input", data)
}

func (h *Handler) HandleFlowExecutionResults(c echo.Context) error {
	data := struct {
		Page
		Flow  models.Flow
		LogID string
	}{
		Page: Page{
			Title: "Flow Execution Results",
		},
	}

	user, ok := c.Get("user").(models.UserInfo)
	if !ok {
		data.ErrMessage = "could not get user details"
		return c.Render(http.StatusBadRequest, "flow_status", data)
	}

	flowID := c.Param("flowID")
	if flowID == "" {
		data.ErrMessage = "flow id cannot be empty"
		return c.Render(http.StatusBadRequest, "flow_status", data)
	}

	logID := c.Param("logID")
	if logID == "" {
		data.ErrMessage = "execution id cannot be empty"
		return c.Render(http.StatusBadRequest, "flow_status", data)
	}

	f, err := h.co.GetFlowByID(flowID)
	if err != nil {
		data.ErrMessage = err.Error()
		return c.Render(http.StatusBadRequest, "flow_status", data)
	}
	data.Flow = f

	exec, err := h.co.GetExecutionSummaryByExecID(c.Request().Context(), logID)
	if err != nil {
		data.ErrMessage = err.Error()
		return c.Render(http.StatusBadRequest, "flow_status", data)
	}

	if exec.TriggeredBy != user.ID {
		data.ErrMessage = "you are not allowed to view this execution summary"
		return c.Render(http.StatusForbidden, "flow_status", data)
	}
	data.LogID = logID

	return c.Render(http.StatusOK, "flow_status", data)
}

func (h *Handler) HandleUserManagementView(c echo.Context) error {
	data := struct {
		Page
	}{
		Page: Page{
			Title: "User Management",
		},
	}

	return c.Render(http.StatusOK, "user_management", data)
}

func (h *Handler) HandleGroupManagementView(c echo.Context) error {
	data := struct {
		Page
	}{
		Page: Page{
			Title: "Group Management",
		},
	}

	return c.Render(http.StatusOK, "group_management", data)
}

// func (h *Handler) HandleExecutionSummary(c echo.Context) error {
// 	user, ok := c.Get("user").(models.UserInfo)
// 	if !ok {
// 		return echo.NewHTTPError(http.StatusForbidden, "could not get user details")
// 	}

// 	flowID := c.Param("flowID")
// 	if flowID == "" {
// 		return echo.NewHTTPError(http.StatusBadRequest, "flow id cannot be empty")
// 	}

// 	f, err := h.co.GetFlowByID(flowID)
// 	if err != nil {
// 		return render(c, partials.InlineError("flow could not be found"), http.StatusNotFound)
// 	}

// 	summary, err := h.co.GetAllExecutionSummary(c.Request().Context(), f, user.ID)
// 	if err != nil {
// 		c.Logger().Error(err)
// 		return render(c, partials.InlineError(err.Error()), http.StatusInternalServerError)
// 	}

// 	return render(c, ui.ExecutionSummaryPage(f, summary), http.StatusOK)
// }
