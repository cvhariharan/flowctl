package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cvhariharan/flowctl/internal/config"
	"github.com/labstack/echo/v4"
)

func newTestHandler(cfg config.Config) *Handler {
	return &Handler{
		logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		config:     cfg,
		authconfig: make(map[string]OIDCAuthConfig),
	}
}

func TestInitOIDCSkipsProviderDiscoveryFailures(t *testing.T) {
	issuer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "discovery failed", http.StatusInternalServerError)
	}))
	defer issuer.Close()

	h := newTestHandler(config.Config{
		App: config.AppConfig{RootURL: "http://localhost:7000"},
		OIDC: []config.OIDCConfig{
			{
				Name:         "broken",
				Issuer:       issuer.URL,
				ClientID:     "client-id",
				ClientSecret: "client-secret",
			},
		},
	})

	if err := h.initOIDC(); err != nil {
		t.Fatalf("initOIDC returned error: %v", err)
	}

	if len(h.authconfig) != 0 {
		t.Fatalf("expected failed provider to be skipped, got %d auth configs", len(h.authconfig))
	}
}

func TestGetSSOProvidersExcludesUnavailableProviders(t *testing.T) {
	h := newTestHandler(config.Config{
		OIDC: []config.OIDCConfig{
			{
				Name:  "broken",
				Label: "Sign in with Broken",
			},
		},
	})

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/sso-providers", nil)
	rec := httptest.NewRecorder()

	if err := h.HandleGetSSOProviders(e.NewContext(req, rec)); err != nil {
		t.Fatalf("HandleGetSSOProviders returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var providers []SSOProvider
	if err := json.Unmarshal(rec.Body.Bytes(), &providers); err != nil {
		t.Fatalf("failed to decode providers response: %v", err)
	}
	if len(providers) != 0 {
		t.Fatalf("expected unavailable provider to be excluded, got %+v", providers)
	}
}

func TestHandleOIDCLoginUnknownProviderDoesNotPanic(t *testing.T) {
	h := newTestHandler(config.Config{})

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/login/oidc/missing", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("provider")
	c.SetParamValues("missing")

	err := h.HandleOIDCLogin(c)
	if err == nil {
		t.Fatal("expected unavailable provider error")
	}

	httpErr, ok := err.(*HTTPError)
	if !ok {
		t.Fatalf("expected HTTPError, got %T", err)
	}
	if httpErr.code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, httpErr.code)
	}
	if httpErr.Error() != fmt.Sprintf("oidc provider is not available: %s", "missing") {
		t.Fatalf("unexpected error message: %s", httpErr.Error())
	}
}
