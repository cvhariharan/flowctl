package mcpserver

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type listVersionsArgs struct {
	Namespace string `json:"namespace" jsonschema:"namespace name"`
	FlowID    string `json:"flow_id" jsonschema:"flow slug"`
	Page      int    `json:"page,omitempty" jsonschema:"1-based page number"`
}

type versionArgs struct {
	Namespace string `json:"namespace" jsonschema:"namespace name"`
	FlowID    string `json:"flow_id" jsonschema:"flow slug"`
	Version   int    `json:"version" jsonschema:"version number from list_flow_versions"`
}

type diffVersionsArgs struct {
	Namespace string `json:"namespace" jsonschema:"namespace name"`
	FlowID    string `json:"flow_id" jsonschema:"flow slug"`
	From      int    `json:"from" jsonschema:"version to diff from"`
	To        int    `json:"to,omitempty" jsonschema:"version to diff to; omit to diff against the active version"`
}

func (s *server) registerVersionTools() {
	addTool(s, &mcp.Tool{
		Name:        "list_flow_versions",
		Description: "List a flow's saved versions, newest first. Every create_flow, update_flow and activate_flow_version produces one. `active` marks the version the flow currently runs.",
		Annotations: readOnly(),
	}, func(ctx context.Context, a *api, in listVersionsArgs) (any, error) {
		body, err := a.get(ctx, apiPath(in.Namespace, "flows", in.FlowID, "versions"), pageQuery(in.Page, ""))
		return result(body), err
	})

	addTool(s, &mcp.Tool{
		Name:        "get_flow_version",
		Description: "Get the source of one flow version, in the format it was saved in.",
		Annotations: readOnly(),
	}, func(ctx context.Context, a *api, in versionArgs) (any, error) {
		body, err := a.get(ctx, apiPath(in.Namespace, "flows", in.FlowID, "versions", strconv.Itoa(in.Version)), nil)
		return result(body), err
	})

	addTool(s, &mcp.Tool{
		Name:        "diff_flow_versions",
		Description: "Line diff of a flow's source between two versions. Each line has an op of equal, insert or delete.",
		Annotations: readOnly(),
	}, func(ctx context.Context, a *api, in diffVersionsArgs) (any, error) {
		q := url.Values{"from": {strconv.Itoa(in.From)}}
		if in.To > 0 {
			q.Set("to", strconv.Itoa(in.To))
		}
		body, err := a.get(ctx, apiPath(in.Namespace, "flows", in.FlowID, "versions", "diff"), q)
		return result(body), err
	})

	addTool(s, &mcp.Tool{
		Name:        "activate_flow_version",
		Description: "Make an earlier version the flow's active definition, for example to roll back a bad update. New executions and schedules use it; running executions keep the version they started with.",
		Annotations: idempotent(),
	}, func(ctx context.Context, a *api, in versionArgs) (any, error) {
		body, err := a.send(ctx, http.MethodPost, apiPath(in.Namespace, "flows", in.FlowID, "versions", strconv.Itoa(in.Version), "activate"), nil)
		return result(body), err
	})
}
