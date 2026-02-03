package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// OpenAPI 3.0 specification for flowctl API
var openAPISpec = map[string]interface{}{
	"openapi": "3.0.3",
	"info": map[string]interface{}{
		"title":       "Flowctl API",
		"description": "API for flowctl - an open-source self-service workflow execution platform",
		"version":     "1.0.0",
		"contact": map[string]string{
			"url": "https://flowctl.net",
		},
		"license": map[string]string{
			"name": "Apache 2.0",
			"url":  "https://www.apache.org/licenses/LICENSE-2.0",
		},
	},
	"servers": []map[string]string{
		{"url": "/api/v1", "description": "API v1"},
	},
	"security": []map[string][]string{
		{"bearerAuth": {}},
		{"apiKeyAuth": {}},
		{"cookieAuth": {}},
	},
	"components": map[string]interface{}{
		"securitySchemes": map[string]interface{}{
			"bearerAuth": map[string]string{
				"type":         "http",
				"scheme":       "bearer",
				"description":  "API key authentication using Bearer token",
			},
			"apiKeyAuth": map[string]interface{}{
				"type": "apiKey",
				"in":   "header",
				"name": "X-API-Key",
				"description": "API key authentication",
			},
			"cookieAuth": map[string]interface{}{
				"type": "apiKey",
				"in":   "cookie",
				"name": "session",
				"description": "Session cookie authentication",
			},
		},
		"schemas": map[string]interface{}{
			"Error": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"error":   map[string]string{"type": "string"},
					"message": map[string]string{"type": "string"},
				},
			},
			"User": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id":         map[string]string{"type": "string", "format": "uuid"},
					"username":   map[string]string{"type": "string"},
					"name":       map[string]string{"type": "string"},
					"login_type": map[string]string{"type": "string"},
					"role":       map[string]string{"type": "string"},
				},
			},
			"APIKey": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id":           map[string]string{"type": "string", "format": "uuid"},
					"name":         map[string]string{"type": "string"},
					"key_prefix":   map[string]string{"type": "string"},
					"expires_at":   map[string]string{"type": "string", "format": "date-time"},
					"last_used_at": map[string]string{"type": "string", "format": "date-time"},
					"created_at":   map[string]string{"type": "string", "format": "date-time"},
				},
			},
			"APIKeyCreate": map[string]interface{}{
				"type":     "object",
				"required": []string{"name"},
				"properties": map[string]interface{}{
					"name":       map[string]string{"type": "string"},
					"expires_in": map[string]string{"type": "string", "description": "30d, 60d, 90d, 180d, 1y, or never"},
				},
			},
			"APIKeyCreateResponse": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id":         map[string]string{"type": "string", "format": "uuid"},
					"name":       map[string]string{"type": "string"},
					"key_prefix": map[string]string{"type": "string"},
					"key":        map[string]string{"type": "string", "description": "The full API key (only shown once)"},
					"expires_at": map[string]string{"type": "string", "format": "date-time"},
					"created_at": map[string]string{"type": "string", "format": "date-time"},
				},
			},
			"Flow": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id":          map[string]string{"type": "string"},
					"slug":        map[string]string{"type": "string"},
					"name":        map[string]string{"type": "string"},
					"description": map[string]string{"type": "string"},
					"step_count":  map[string]string{"type": "integer"},
				},
			},
			"FlowTriggerResponse": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"exec_id":      map[string]string{"type": "string"},
					"scheduled_at": map[string]string{"type": "string", "format": "date-time"},
				},
			},
			"Execution": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id":                map[string]string{"type": "string"},
					"flow_name":         map[string]string{"type": "string"},
					"flow_id":           map[string]string{"type": "string"},
					"status":            map[string]string{"type": "string"},
					"trigger_type":      map[string]string{"type": "string"},
					"triggered_by":      map[string]string{"type": "string"},
					"current_action_id": map[string]string{"type": "string"},
					"created_at":        map[string]string{"type": "string", "format": "date-time"},
					"completed_at":      map[string]string{"type": "string", "format": "date-time"},
				},
			},
		},
	},
	"paths": map[string]interface{}{
		"/api-keys": map[string]interface{}{
			"get": map[string]interface{}{
				"summary":     "List API keys",
				"description": "List all API keys for the authenticated user",
				"tags":        []string{"API Keys"},
				"responses": map[string]interface{}{
					"200": map[string]interface{}{
						"description": "List of API keys",
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"api_keys": map[string]interface{}{
											"type":  "array",
											"items": map[string]string{"$ref": "#/components/schemas/APIKey"},
										},
									},
								},
							},
						},
					},
				},
			},
			"post": map[string]interface{}{
				"summary":     "Create API key",
				"description": "Create a new API key for the authenticated user",
				"tags":        []string{"API Keys"},
				"requestBody": map[string]interface{}{
					"required": true,
					"content": map[string]interface{}{
						"application/json": map[string]interface{}{
							"schema": map[string]string{"$ref": "#/components/schemas/APIKeyCreate"},
						},
					},
				},
				"responses": map[string]interface{}{
					"201": map[string]interface{}{
						"description": "API key created",
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]string{"$ref": "#/components/schemas/APIKeyCreateResponse"},
							},
						},
					},
				},
			},
		},
		"/api-keys/{keyId}": map[string]interface{}{
			"delete": map[string]interface{}{
				"summary":     "Delete API key",
				"description": "Delete an API key",
				"tags":        []string{"API Keys"},
				"parameters": []map[string]interface{}{
					{
						"name":     "keyId",
						"in":       "path",
						"required": true,
						"schema":   map[string]string{"type": "string", "format": "uuid"},
					},
				},
				"responses": map[string]interface{}{
					"200": map[string]string{"description": "API key deleted"},
				},
			},
		},
		"/users/profile": map[string]interface{}{
			"get": map[string]interface{}{
				"summary":     "Get user profile",
				"description": "Get the profile of the authenticated user",
				"tags":        []string{"Users"},
				"responses": map[string]interface{}{
					"200": map[string]interface{}{
						"description": "User profile",
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]string{"$ref": "#/components/schemas/User"},
							},
						},
					},
				},
			},
		},
		"/{namespace}/flows": map[string]interface{}{
			"get": map[string]interface{}{
				"summary":     "List flows",
				"description": "List all flows in a namespace",
				"tags":        []string{"Flows"},
				"parameters": []map[string]interface{}{
					{
						"name":     "namespace",
						"in":       "path",
						"required": true,
						"schema":   map[string]string{"type": "string"},
					},
				},
				"responses": map[string]interface{}{
					"200": map[string]interface{}{
						"description": "List of flows",
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"flows": map[string]interface{}{
											"type":  "array",
											"items": map[string]string{"$ref": "#/components/schemas/Flow"},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		"/{namespace}/trigger/{flow}": map[string]interface{}{
			"post": map[string]interface{}{
				"summary":     "Trigger flow execution",
				"description": "Trigger a flow execution with optional inputs",
				"tags":        []string{"Flows"},
				"parameters": []map[string]interface{}{
					{
						"name":     "namespace",
						"in":       "path",
						"required": true,
						"schema":   map[string]string{"type": "string"},
					},
					{
						"name":     "flow",
						"in":       "path",
						"required": true,
						"schema":   map[string]string{"type": "string"},
					},
				},
				"requestBody": map[string]interface{}{
					"content": map[string]interface{}{
						"application/json": map[string]interface{}{
							"schema": map[string]interface{}{
								"type":                 "object",
								"additionalProperties": true,
							},
						},
					},
				},
				"responses": map[string]interface{}{
					"200": map[string]interface{}{
						"description": "Flow triggered",
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]string{"$ref": "#/components/schemas/FlowTriggerResponse"},
							},
						},
					},
				},
			},
		},
		"/{namespace}/flows/executions": map[string]interface{}{
			"get": map[string]interface{}{
				"summary":     "List executions",
				"description": "List all executions in a namespace",
				"tags":        []string{"Executions"},
				"parameters": []map[string]interface{}{
					{
						"name":     "namespace",
						"in":       "path",
						"required": true,
						"schema":   map[string]string{"type": "string"},
					},
				},
				"responses": map[string]interface{}{
					"200": map[string]interface{}{
						"description": "List of executions",
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"executions": map[string]interface{}{
											"type":  "array",
											"items": map[string]string{"$ref": "#/components/schemas/Execution"},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	},
}

// HandleOpenAPISpec returns the OpenAPI 3.0 specification
func (h *Handler) HandleOpenAPISpec(c echo.Context) error {
	return c.JSON(http.StatusOK, openAPISpec)
}
