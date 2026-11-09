package handlers

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cvhariharan/flowctl/internal/config"
	"github.com/cvhariharan/flowctl/internal/core"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

func TestNodeMutationValidationStatus(t *testing.T) {
	for _, operation := range []string{"create", "update"} {
		for _, tt := range []struct{ name, value string }{
			{"Name", "local"}, {"Name", "uploads"}, {"Name", "global"},
			{"Name", "bad-name"}, {"Hostname", "foo bar;baz"}, {"Hostname", ""},
			{"Username", "a"}, {"ConnectionType", "telnet"},
		} {
			t.Run(operation+"/"+tt.name+"/"+tt.value, func(t *testing.T) {
				n := NodeReq{Name: "valid", Hostname: "localhost", Port: 22, Username: "admin", ConnectionType: "ssh", Auth: NodeAuth{Method: "private_key", CredentialID: uuid.NewString()}}
				switch tt.name {
				case "Name":
					n.Name = tt.value
				case "Hostname":
					n.Hostname = tt.value
				case "Username":
					n.Username = tt.value
				case "ConnectionType":
					n.ConnectionType = tt.value
				}
				body, err := json.Marshal(n)
				if err != nil {
					t.Fatal(err)
				}
				req := httptest.NewRequest(http.MethodPost, "/nodes", bytes.NewReader(body))
				req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
				rec := httptest.NewRecorder()
				e := echo.New()
				ctx := e.NewContext(req, rec)
				ctx.Set("namespace", uuid.NewString())
				ctx.SetParamNames("nodeID")
				ctx.SetParamValues(uuid.NewString())
				h := newTestHandler(config.Config{})
				h.co = &core.Core{}
				if operation == "create" {
					err = h.HandleCreateNode(ctx)
				} else {
					err = h.HandleUpdateNode(ctx)
				}
				if err == nil {
					t.Fatal("expected validation error")
				}
				h.ErrorHandler(err, ctx)
				if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), ErrValidationFailed) {
					t.Fatalf("status=%d, response=%s", rec.Code, rec.Body.String())
				}
			})
		}
	}
}

func TestImportNodesInvalidFileStatus(t *testing.T) {
	for _, tt := range []struct{ name, input, message string }{
		{"empty", "", "could not read csv header"},
		{"header only", "name,hostname,port,username,connection_type,credential,tags\n", "no rows found"},
		{"missing header", "web,localhost,22,admin,ssh,my_ssh_key,\n", "expected column 1"},
		{"short header", "name,hostname\n", "csv must have columns"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var body bytes.Buffer
			writer := multipart.NewWriter(&body)
			part, err := writer.CreateFormFile("file", "nodes.csv")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := part.Write([]byte(tt.input)); err != nil {
				t.Fatal(err)
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest(http.MethodPost, "/nodes/import", &body)
			req.Header.Set(echo.HeaderContentType, writer.FormDataContentType())
			rec := httptest.NewRecorder()
			ctx := echo.New().NewContext(req, rec)
			ctx.Set("namespace", uuid.NewString())
			h := newTestHandler(config.Config{App: config.AppConfig{MaxNodeImportSize: 1024}})
			h.co = &core.Core{}
			err = h.HandleImportNodes(ctx)
			if err == nil {
				t.Fatal("expected file validation error")
			}
			h.ErrorHandler(err, ctx)
			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), tt.message) {
				t.Fatalf("status=%d, response=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestImportNodesMissingRowCredential(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "nodes.csv")
	if err != nil {
		t.Fatal(err)
	}
	input := "name,hostname,port,username,connection_type,credential,tags\n" +
		"web,localhost,22,admin,ssh,,\n"
	if _, err := part.Write([]byte(input)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/nodes/import", &body)
	req.Header.Set(echo.HeaderContentType, writer.FormDataContentType())
	rec := httptest.NewRecorder()
	ctx := echo.New().NewContext(req, rec)
	ctx.Set("namespace", uuid.NewString())
	h := newTestHandler(config.Config{App: config.AppConfig{MaxNodeImportSize: 1024}})
	h.co = &core.Core{}
	if err := h.HandleImportNodes(ctx); err != nil {
		t.Fatal(err)
	}
	var result NodeImportResp
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK || result.Imported != 0 || len(result.Errors) != 1 || result.Errors[0].Row != 2 || result.Errors[0].Name != "web" || result.Errors[0].Error != "credential is required" {
		t.Fatalf("status=%d, response=%s", rec.Code, rec.Body.String())
	}
}
