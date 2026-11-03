package mcpserver

import (
	"context"
	"net/http"
	"reflect"
	"strings"

	"github.com/cvhariharan/flowctl/internal/handlers"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const authoringLoop = "To author a flow: list_executors, get_executor_schema for each executor you use, list_nodes and list_credentials to fill `on` and credential references, then create_flow, then get_flow and trigger_flow to test it."

type executorArgs struct {
	Executor string `json:"executor" jsonschema:"executor name from list_executors"`
}

type listArgs struct {
	Namespace string `json:"namespace" jsonschema:"namespace name"`
	Query     string `json:"query,omitempty" jsonschema:"filter by name"`
	Page      int    `json:"page,omitempty" jsonschema:"1-based page number"`
}

type createFlowArgs struct {
	Namespace  string                 `json:"namespace" jsonschema:"namespace name"`
	Definition handlers.FlowCreateReq `json:"definition"`
}

type updateFlowArgs struct {
	Namespace  string                 `json:"namespace" jsonschema:"namespace name"`
	FlowID     string                 `json:"flow_id" jsonschema:"flow slug"`
	Definition handlers.FlowUpdateReq `json:"definition"`
}

func (s *server) registerAuthorTools() {
	addTool(s, &mcp.Tool{
		Name:        "list_executors",
		Description: "List the executors a flow action can use. " + authoringLoop,
		Annotations: readOnly(),
	}, func(ctx context.Context, a *api, _ struct{}) (any, error) {
		body, err := a.get(ctx, "/api/v1/executors", nil)
		return result(body), err
	})

	addTool(s, &mcp.Tool{
		Name:        "get_executor_schema",
		Description: "Get the JSON schema of an executor's `with` config for a flow action.",
		Annotations: readOnly(),
	}, func(ctx context.Context, a *api, in executorArgs) (any, error) {
		body, err := a.get(ctx, apiPath("executors", in.Executor, "config"), nil)
		return result(body), err
	})

	addTool(s, &mcp.Tool{
		Name:        "list_nodes",
		Description: "List the nodes in a namespace. Node names go in an action's `on` list.",
		Annotations: readOnly(),
	}, func(ctx context.Context, a *api, in listArgs) (any, error) {
		body, err := a.get(ctx, apiPath(in.Namespace, "nodes"), pageQuery(in.Page, in.Query))
		return result(body), err
	})

	addTool(s, &mcp.Tool{
		Name:        "list_credentials",
		Description: "List the credentials in a namespace by name and key type. Key material is never returned.",
		Annotations: readOnly(),
	}, func(ctx context.Context, a *api, in listArgs) (any, error) {
		body, err := a.get(ctx, apiPath(in.Namespace, "credentials"), pageQuery(in.Page, in.Query))
		return result(body), err
	})

	addTool(s, &mcp.Tool{
		Name:        "get_flow_definition",
		Description: "Get a flow's full definition in the same shape create_flow takes.",
		Annotations: readOnly(),
	}, func(ctx context.Context, a *api, in flowArgs) (any, error) {
		body, err := a.get(ctx, apiPath(in.Namespace, "flows", in.FlowID, "config"), nil)
		return result(body), err
	})

	addTool(s, &mcp.Tool{
		Name:        "create_flow",
		Description: "Create a flow. The flow slug is generated from metadata.name and returned as `id`. " + authoringLoop,
		InputSchema: definitionSchema[createFlowArgs, handlers.FlowCreateReq](),
	}, func(ctx context.Context, a *api, in createFlowArgs) (any, error) {
		body, err := a.send(ctx, http.MethodPost, apiPath(in.Namespace, "flows"), in.Definition)
		return result(body), err
	})

	addTool(s, &mcp.Tool{
		Name:        "update_flow",
		Description: "Replace a flow's definition. Start from get_flow_definition: keep inputs, actions and notify, and move prefix, description, schedules, allow_overlap, user_schedulable, max_retries and execution_mode out of metadata to the top level. The name and slug cannot change. Each update creates a new flow version that can be rolled back from the UI.",
		InputSchema: definitionSchema[updateFlowArgs, handlers.FlowUpdateReq](),
		Annotations: idempotent(),
	}, func(ctx context.Context, a *api, in updateFlowArgs) (any, error) {
		body, err := a.send(ctx, http.MethodPut, apiPath(in.Namespace, "flows", in.FlowID), in.Definition)
		return result(body), err
	})

	addTool(s, &mcp.Tool{
		Name:        "delete_flow",
		Description: "Delete a flow by slug from a namespace.",
		Annotations: destructive(true),
	}, func(ctx context.Context, a *api, in flowArgs) (any, error) {
		body, err := a.send(ctx, http.MethodDelete, apiPath(in.Namespace, "flows", in.FlowID), nil)
		return result(body), err
	})
}

func definitionSchema[In, Def any]() *jsonschema.Schema {
	s, err := jsonschema.For[In](nil)
	if err != nil {
		panic(err)
	}
	applyValidateTags(reflect.TypeFor[Def](), s.Properties["definition"])
	return s
}

func applyValidateTags(t reflect.Type, s *jsonschema.Schema) {
	if s == nil {
		return
	}

	switch t.Kind() {
	case reflect.Pointer:
		applyValidateTags(t.Elem(), s)
	case reflect.Slice, reflect.Array:
		applyValidateTags(t.Elem(), s.Items)
	case reflect.Map:
		applyValidateTags(t.Elem(), s.AdditionalProperties)
	case reflect.Struct:
		s.Required = nil
		for i := range t.NumField() {
			f := t.Field(i)
			name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
			prop := s.Properties[name]
			if prop == nil {
				continue
			}

			field, elem, _ := strings.Cut(f.Tag.Get("validate"), ",dive")
			if hasRule(field, "required") {
				s.Required = append(s.Required, name)
			}
			setEnum(prop, field)
			if prop.Items != nil {
				setEnum(prop.Items, elem)
			}
			applyValidateTags(f.Type, prop)
		}
	}
}

func hasRule(rules, rule string) bool {
	for r := range strings.SplitSeq(rules, ",") {
		if r == rule {
			return true
		}
	}
	return false
}

func setEnum(s *jsonschema.Schema, rules string) {
	for r := range strings.SplitSeq(rules, ",") {
		values, ok := strings.CutPrefix(r, "oneof=")
		if !ok {
			continue
		}
		if hasRule(rules, "omitempty") {
			s.Enum = append(s.Enum, "")
		}
		for v := range strings.FieldsSeq(values) {
			s.Enum = append(s.Enum, v)
		}
	}
}
