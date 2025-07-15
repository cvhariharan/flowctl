package handlers

import (
	"encoding/json"
	"html/template"
	"io"
	"net/http"
	"fmt"

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
	Namespace  string
}

func (h *Handler) HandleLoginView(c echo.Context) error {
	return c.Render(http.StatusOK, "login", nil)
}

func (h *Handler) HandleFlowsListView(c echo.Context) error {
	namespace := c.Param("namespace")
	data := struct {
		Page
		Flows []models.Flow
	}{
		Page: Page{
			Title: "Flows",
			Namespace: namespace,
		},
	}

	namespaceID, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(http.StatusBadRequest, "could not get namespace", nil, nil)
	}

	flows, err := h.co.GetAllFlows(c.Request().Context(), namespaceID)
	if err != nil {
		data.ErrMessage = err.Error()
		return c.Render(http.StatusBadRequest, "flows", data)
	}

	data.Flows = flows
	return c.Render(http.StatusOK, "flows", data)
}

func (h *Handler) HandleFlowFormView(c echo.Context) error {
	namespace := c.Param("namespace")

	data := struct {
		Page
		Flow        models.Flow
		InputErrors map[string]string
	}{
		Page: Page{
			Title: "Flow Input",
			Namespace: namespace,
		},
	}

	namespaceID, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(http.StatusBadRequest, "could not get namespace", nil, nil)
	}

	flow, err := h.co.GetFlowByID(c.Param("flow"), namespaceID)
	if err != nil {
		data.ErrMessage = err.Error()
		return c.Render(http.StatusBadRequest, "flow_input", data)
	}
	data.Flow = flow

	return c.Render(http.StatusOK, "flow_input", data)
}

func (h *Handler) HandleFlowExecutionResults(c echo.Context) error {
	namespace := c.Param("namespace")

	data := struct {
		Page
		Flow  models.Flow
		LogID string
	}{
		Page: Page{
			Title: "Flow Execution Results",
			Namespace: namespace,
		},
	}

	namespaceID, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(http.StatusBadRequest, "could not get namespace", nil, nil)
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

	f, err := h.co.GetFlowByID(flowID, namespaceID)
	if err != nil {
		data.ErrMessage = err.Error()
		return c.Render(http.StatusBadRequest, "flow_status", data)
	}
	data.Flow = f

	exec, err := h.co.GetExecutionSummaryByExecID(c.Request().Context(), logID, namespaceID)
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

func (h *Handler) HandleApprovalView(c echo.Context) error {
	namespace := c.Param("namespace")
	data := struct {
		Page
		Request struct {
			ID string
			Title string
			Description string
			RequestedBy string
		}
	}{
		Page: Page{
			Title: "Approval Requests",
			Namespace: namespace,
		},
	}

	namespaceID, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(http.StatusBadRequest, "could not get namespace", nil, nil)
	}

	approvalID := c.Param("approvalID")
	if approvalID == "" {
		data.ErrMessage = "approval ID cannot be empty"
		return c.Render(http.StatusBadRequest, "approval", data)
	}

	areq, err := h.co.GetApprovalRequest(c.Request().Context(), approvalID, namespaceID)
	if err != nil {
		data.ErrMessage = err.Error()
		return c.Render(http.StatusBadRequest, "approval", data)
	}

	if areq.Status != models.ApprovalStatusPending {
		data.ErrMessage = "request has already been processed"
		return c.Render(http.StatusBadRequest, "approval", data)
	}

	f, err := h.co.GetFlowFromLogID(areq.ExecID, namespaceID)
	if err != nil {
		data.ErrMessage = err.Error()
		return c.Render(http.StatusBadRequest, "approval", data)
	}

	data.Request.ID = approvalID
	data.Request.Title = f.Meta.Name
	data.Request.Description = fmt.Sprintf("Approval request to execute action %q from flow %q", areq.ActionID, f.Meta.Name)
	data.Request.RequestedBy = areq.RequestedBy

	return c.Render(http.StatusOK, "approval", data)
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
