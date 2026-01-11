package webhooks

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
	"time"
)

const TimeFormat = "2006-01-02T15:04:05Z"

type TemplateContext struct {
	FlowID                string
	FlowName              string
	FlowSlug              string
	FlowURL               string
	ExecutionID           string
	ExecutionStatus       string
	ExecutionTriggerType  string
	ExecutionTriggeredBy  string
	ExecutionStartedAt    string
	ExecutionCompletedAt  string
	ExecutionError        string
	NamespaceID           string
	NamespaceName         string
	RootURL               string
}

func BuildTemplateData(ctx TemplateContext) map[string]any {
	return map[string]any{
		"flow": map[string]any{
			"id":   ctx.FlowID,
			"name": ctx.FlowName,
			"slug": ctx.FlowSlug,
			"url":  ctx.FlowURL,
		},
		"execution": map[string]any{
			"id":           ctx.ExecutionID,
			"status":       ctx.ExecutionStatus,
			"trigger_type": ctx.ExecutionTriggerType,
			"triggered_by": ctx.ExecutionTriggeredBy,
			"started_at":   ctx.ExecutionStartedAt,
			"completed_at": ctx.ExecutionCompletedAt,
			"error":        ctx.ExecutionError,
		},
		"namespace": map[string]any{
			"id":   ctx.NamespaceID,
			"name": ctx.NamespaceName,
		},
		"app": map[string]any{
			"root_url": ctx.RootURL,
		},
	}
}

func RenderTemplate(body string, data map[string]any) (string, error) {
	tmpl, err := template.New("webhook").Option("missingkey=zero").Parse(body)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func SampleTemplateData(rootURL string) map[string]any {
	now := time.Now().UTC()
	namespaceName := "default"
	flowSlug := "sample-flow"
	ctx := TemplateContext{
		FlowID:               flowSlug,
		FlowName:             "Sample Flow",
		FlowSlug:             flowSlug,
		FlowURL:              BuildFlowURL(rootURL, namespaceName, flowSlug),
		ExecutionID:          "exec-sample-123",
		ExecutionStatus:      "completed",
		ExecutionTriggerType: "manual",
		ExecutionTriggeredBy: "system@example.com",
		ExecutionStartedAt:   now.Add(-2 * time.Minute).Format(TimeFormat),
		ExecutionCompletedAt: now.Format(TimeFormat),
		ExecutionError:       "",
		NamespaceID:          "00000000-0000-0000-0000-000000000000",
		NamespaceName:        namespaceName,
		RootURL:              rootURL,
	}
	return BuildTemplateData(ctx)
}

func BuildFlowURL(rootURL string, namespaceName string, flowID string) string {
	root := strings.TrimRight(rootURL, "/")
	if root == "" {
		return fmt.Sprintf("/view/%s/flows/%s", namespaceName, flowID)
	}
	return fmt.Sprintf("%s/view/%s/flows/%s", root, namespaceName, flowID)
}
