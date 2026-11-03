package mcpserver

import (
	"context"
	"net/http"

	"github.com/cvhariharan/flowctl/internal/handlers"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type listSchedulesArgs struct {
	Namespace string `json:"namespace" jsonschema:"namespace name"`
	FlowID    string `json:"flow_id" jsonschema:"flow slug"`
	Page      int    `json:"page,omitempty" jsonschema:"1-based page number"`
}

type createScheduleArgs struct {
	Namespace string         `json:"namespace" jsonschema:"namespace name"`
	FlowID    string         `json:"flow_id" jsonschema:"flow slug"`
	Name      string         `json:"name,omitempty" jsonschema:"schedule name"`
	Cron      string         `json:"cron" jsonschema:"cron expression, e.g. 0 2 * * *"`
	Timezone  string         `json:"timezone" jsonschema:"IANA timezone, e.g. UTC or Asia/Kolkata"`
	Inputs    map[string]any `json:"inputs,omitempty" jsonschema:"flow input values used for each scheduled run"`
}

type updateScheduleArgs struct {
	Namespace  string         `json:"namespace" jsonschema:"namespace name"`
	FlowID     string         `json:"flow_id" jsonschema:"flow slug"`
	ScheduleID string         `json:"schedule_id" jsonschema:"schedule uuid from list_schedules"`
	Name       *string        `json:"name,omitempty" jsonschema:"schedule name; omit to keep the current name"`
	Cron       string         `json:"cron" jsonschema:"cron expression, e.g. 0 2 * * *"`
	Timezone   string         `json:"timezone" jsonschema:"IANA timezone, e.g. UTC or Asia/Kolkata"`
	Inputs     map[string]any `json:"inputs,omitempty" jsonschema:"flow input values used for each scheduled run"`
	IsActive   bool           `json:"is_active" jsonschema:"whether the schedule runs"`
}

type scheduleArgs struct {
	Namespace  string `json:"namespace" jsonschema:"namespace name"`
	FlowID     string `json:"flow_id" jsonschema:"flow slug"`
	ScheduleID string `json:"schedule_id" jsonschema:"schedule uuid from list_schedules"`
}

func (s *server) registerScheduleTools() {
	addTool(s, &mcp.Tool{
		Name:        "list_schedules",
		Description: "List the cron schedules on a flow.",
		Annotations: readOnly(),
	}, func(ctx context.Context, a *api, in listSchedulesArgs) (any, error) {
		body, err := a.get(ctx, apiPath(in.Namespace, "flows", in.FlowID, "schedules"), pageQuery(in.Page, ""))
		return result(body), err
	})

	addTool(s, &mcp.Tool{
		Name:        "create_schedule",
		Description: "Add a cron schedule to a flow. Inputs use the same names as trigger_flow; call get_flow to learn them.",
	}, func(ctx context.Context, a *api, in createScheduleArgs) (any, error) {
		req := handlers.ScheduleCreateReq{Name: in.Name, Cron: in.Cron, Timezone: in.Timezone, Inputs: inputsOrEmpty(in.Inputs)}
		body, err := a.send(ctx, http.MethodPost, apiPath(in.Namespace, "flows", in.FlowID, "schedules"), req)
		return result(body), err
	})

	addTool(s, &mcp.Tool{
		Name:        "update_schedule",
		Description: "Replace a schedule's cron, timezone, inputs and active state.",
		Annotations: idempotent(),
	}, func(ctx context.Context, a *api, in updateScheduleArgs) (any, error) {
		req := handlers.ScheduleUpdateReq{Name: in.Name, Cron: in.Cron, Timezone: in.Timezone, Inputs: inputsOrEmpty(in.Inputs), IsActive: in.IsActive}
		body, err := a.send(ctx, http.MethodPut, apiPath(in.Namespace, "flows", in.FlowID, "schedules", in.ScheduleID), req)
		return result(body), err
	})

	addTool(s, &mcp.Tool{
		Name:        "delete_schedule",
		Description: "Delete a schedule from a flow.",
		Annotations: destructive(true),
	}, func(ctx context.Context, a *api, in scheduleArgs) (any, error) {
		body, err := a.send(ctx, http.MethodDelete, apiPath(in.Namespace, "flows", in.FlowID, "schedules", in.ScheduleID), nil)
		return result(body), err
	})
}

func inputsOrEmpty(inputs map[string]any) map[string]any {
	if inputs == nil {
		return map[string]any{}
	}
	return inputs
}
