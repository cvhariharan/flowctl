package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/cvhariharan/flowctl/internal/config"
	"github.com/labstack/echo/v4"
	"github.com/zerodha/simplesessions/v3"
	"golang.org/x/oauth2"
)

// fakeStoreErr is a sentinel error used by fakeStore.
type fakeStoreErr struct{ msg string }

func (e *fakeStoreErr) Error() string { return e.msg }

// fakeStore is an in-memory simplesessions.Store for testing.
type fakeStore struct {
	data map[string]map[string]interface{}
}

func (s *fakeStore) Create(id string) error {
	s.data[id] = make(map[string]interface{})
	return nil
}

func (s *fakeStore) Get(id, key string) (interface{}, error) {
	sess, ok := s.data[id]
	if !ok {
		return nil, &fakeStoreErr{"session not found"}
	}
	v, ok := sess[key]
	if !ok {
		return nil, &fakeStoreErr{"key not found"}
	}
	return v, nil
}

func (s *fakeStore) GetMulti(id string, keys ...string) (map[string]interface{}, error) {
	sess, ok := s.data[id]
	if !ok {
		return nil, &fakeStoreErr{"session not found"}
	}
	out := make(map[string]interface{}, len(keys))
	for _, k := range keys {
		out[k] = sess[k]
	}
	return out, nil
}

func (s *fakeStore) GetAll(id string) (map[string]interface{}, error) {
	sess, ok := s.data[id]
	if !ok {
		return nil, &fakeStoreErr{"session not found"}
	}
	out := make(map[string]interface{}, len(sess))
	for k, v := range sess {
		out[k] = v
	}
	return out, nil
}

func (s *fakeStore) Set(id, key string, val interface{}) error {
	sess, ok := s.data[id]
	if !ok {
		return &fakeStoreErr{"session not found"}
	}
	sess[key] = val
	return nil
}

func (s *fakeStore) SetMulti(id string, data map[string]interface{}) error {
	sess, ok := s.data[id]
	if !ok {
		return &fakeStoreErr{"session not found"}
	}
	for k, v := range data {
		sess[k] = v
	}
	return nil
}

func (s *fakeStore) Delete(id string, keys ...string) error {
	sess, ok := s.data[id]
	if !ok {
		return nil
	}
	for _, k := range keys {
		delete(sess, k)
	}
	return nil
}

func (s *fakeStore) Clear(id string) error {
	if _, ok := s.data[id]; ok {
		s.data[id] = make(map[string]interface{})
	}
	return nil
}

func (s *fakeStore) Destroy(id string) error {
	delete(s.data, id)
	return nil
}

func (s *fakeStore) Int(r interface{}, err error) (int, error) {
	if err != nil {
		return 0, err
	}
	switch v := r.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	}
	return 0, fmt.Errorf("cannot convert %T to int", r)
}

func (s *fakeStore) Int64(r interface{}, err error) (int64, error) {
	if err != nil {
		return 0, err
	}
	if v, ok := r.(int64); ok {
		return v, nil
	}
	return 0, fmt.Errorf("cannot convert %T to int64", r)
}

func (s *fakeStore) UInt64(r interface{}, err error) (uint64, error) {
	if err != nil {
		return 0, err
	}
	if v, ok := r.(uint64); ok {
		return v, nil
	}
	return 0, fmt.Errorf("cannot convert %T to uint64", r)
}

func (s *fakeStore) Float64(r interface{}, err error) (float64, error) {
	if err != nil {
		return 0, err
	}
	if v, ok := r.(float64); ok {
		return v, nil
	}
	return 0, fmt.Errorf("cannot convert %T to float64", r)
}

func (s *fakeStore) String(r interface{}, err error) (string, error) {
	if err != nil {
		return "", err
	}
	if v, ok := r.(string); ok {
		return v, nil
	}
	return "", fmt.Errorf("cannot convert %T to string", r)
}

func (s *fakeStore) Bytes(r interface{}, err error) ([]byte, error) {
	if err != nil {
		return nil, err
	}
	if v, ok := r.([]byte); ok {
		return v, nil
	}
	return nil, fmt.Errorf("cannot convert %T to []byte", r)
}

func (s *fakeStore) Bool(r interface{}, err error) (bool, error) {
	if err != nil {
		return false, err
	}
	if v, ok := r.(bool); ok {
		return v, nil
	}
	return false, fmt.Errorf("cannot convert %T to bool", r)
}

// newTestSessMgr returns a simplesessions.Manager backed by fakeStore and
// pre-seeded with a session whose ID is returned as the second value.
func newTestSessMgr(data map[string]interface{}) (*simplesessions.Manager, string) {
	store := &fakeStore{data: make(map[string]map[string]interface{})}
	id := "test-session-id"
	_ = store.Create(id)
	for k, v := range data {
		store.data[id][k] = v
	}

	mgr := simplesessions.New(simplesessions.Options{
		EnableAutoCreate: false,
		Cookie: simplesessions.CookieOptions{
			IsHTTPOnly: true,
			SameSite:   http.SameSiteLaxMode,
		},
	})

	mgr.SetCookieHooks(getCookie, setCookie)
	mgr.UseStore(store)
	return mgr, id
}

// newFakeOIDCServer returns an httptest.Server that serves a minimal OIDC
// discovery document.  endSessionEndpoint is the value to advertise in the
// document (may be empty).
func newFakeOIDCServer(t *testing.T, endSessionEndpoint string) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/openid-configuration" {
			doc := map[string]interface{}{
				"issuer":                                srv.URL,
				"authorization_endpoint":                srv.URL + "/auth",
				"token_endpoint":                        srv.URL + "/token",
				"jwks_uri":                              srv.URL + "/jwks",
				"response_types_supported":              []string{"code"},
				"subject_types_supported":               []string{"public"},
				"id_token_signing_alg_values_supported": []string{"RS256"},
			}
			if endSessionEndpoint != "" {
				doc["end_session_endpoint"] = endSessionEndpoint
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(doc)
			return
		}
		if r.URL.Path == "/jwks" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"keys":[]}`))
			return
		}
		http.NotFound(w, r)
	}))
	return srv
}

// ---------------------------------------------------------------------------

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

// TestInitOIDCReadsEndSessionEndpoint verifies that initOIDC populates
// endSessionEndpoint from the OIDC discovery document.
func TestInitOIDCReadsEndSessionEndpoint(t *testing.T) {
	srv := newFakeOIDCServer(t, "")
	defer srv.Close()

	endSession := srv.URL + "/end-session"
	// Restart with end_session_endpoint advertised.
	srv2 := newFakeOIDCServer(t, endSession)
	defer srv2.Close()

	h := newTestHandler(config.Config{
		App: config.AppConfig{RootURL: "http://localhost:7000"},
		OIDC: []config.OIDCConfig{
			{
				Name:         "test",
				Issuer:       srv2.URL,
				ClientID:     "client-id",
				ClientSecret: "client-secret",
			},
		},
	})

	if err := h.initOIDC(); err != nil {
		t.Fatalf("initOIDC returned error: %v", err)
	}

	cfg, ok := h.authconfig["test"]
	if !ok {
		t.Fatal("expected authconfig entry for 'test'")
	}
	if cfg.endSessionEndpoint != endSession {
		t.Fatalf("expected endSessionEndpoint %q, got %q", endSession, cfg.endSessionEndpoint)
	}
}

// TestInitOIDCLogoutURLOverridesDiscovery verifies that a configured LogoutURL
// takes precedence over the value in the discovery document.
func TestInitOIDCLogoutURLOverridesDiscovery(t *testing.T) {
	discoveredEndSession := "https://discovered.example.com/end-session"
	srv := newFakeOIDCServer(t, discoveredEndSession)
	defer srv.Close()

	configuredLogoutURL := "https://custom.example.com/logout"

	h := newTestHandler(config.Config{
		App: config.AppConfig{RootURL: "http://localhost:7000"},
		OIDC: []config.OIDCConfig{
			{
				Name:         "test",
				Issuer:       srv.URL,
				ClientID:     "client-id",
				ClientSecret: "client-secret",
				LogoutURL:    configuredLogoutURL,
			},
		},
	})

	if err := h.initOIDC(); err != nil {
		t.Fatalf("initOIDC returned error: %v", err)
	}

	cfg, ok := h.authconfig["test"]
	if !ok {
		t.Fatal("expected authconfig entry for 'test'")
	}
	if cfg.endSessionEndpoint != configuredLogoutURL {
		t.Fatalf("expected endSessionEndpoint %q, got %q", configuredLogoutURL, cfg.endSessionEndpoint)
	}
}

// TestHandleLogoutOIDCSetsXRedirectHeader verifies that when an OIDC session
// is destroyed, HandleLogout sets an x-redirect header pointing to the IdP
// end-session endpoint.
func TestHandleLogoutOIDCSetsXRedirectHeader(t *testing.T) {
	endSessionEndpoint := "https://idp.example.com/end-session"
	rawIDToken := "raw.id.token"

	td := TokenData{Provider: "test", RawIDToken: rawIDToken}
	encoded, err := td.GetEncoded()
	if err != nil {
		t.Fatalf("failed to encode token data: %v", err)
	}

	sessMgr, sessID := newTestSessMgr(map[string]interface{}{
		"method":   "oidc",
		"id_token": encoded,
	})

	h := &Handler{
		logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		config:     config.Config{App: config.AppConfig{RootURL: "http://localhost:7000"}},
		authconfig: map[string]OIDCAuthConfig{
			"test": {
				endSessionEndpoint: endSessionEndpoint,
				oauth2Config: &oauth2.Config{ClientID: "client-id"},
			},
		},
		sessMgr: sessMgr,
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: sessID})
	rec := httptest.NewRecorder()

	if err := h.HandleLogout(e.NewContext(req, rec)); err != nil {
		t.Fatalf("HandleLogout returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	xRedirect := rec.Header().Get("x-redirect")
	if xRedirect == "" {
		t.Fatal("expected x-redirect header to be set")
	}

	parsed, err := url.Parse(xRedirect)
	if err != nil {
		t.Fatalf("x-redirect is not a valid URL: %v", err)
	}

	if !strings.HasPrefix(xRedirect, endSessionEndpoint) {
		t.Fatalf("x-redirect %q does not start with endSessionEndpoint %q", xRedirect, endSessionEndpoint)
	}
	q := parsed.Query()
	if q.Get("id_token_hint") != rawIDToken {
		t.Fatalf("expected id_token_hint=%q, got %q", rawIDToken, q.Get("id_token_hint"))
	}
	if q.Get("client_id") != "client-id" {
		t.Fatalf("expected client_id=%q, got %q", "client-id", q.Get("client_id"))
	}
}

// TestHandleLogoutOIDCWithoutEndSessionEndpointNoRedirect verifies that when
// the OIDC provider has no end-session endpoint, no x-redirect is set.
func TestHandleLogoutOIDCWithoutEndSessionEndpointNoRedirect(t *testing.T) {
	td := TokenData{Provider: "test", RawIDToken: "raw.id.token"}
	encoded, err := td.GetEncoded()
	if err != nil {
		t.Fatalf("failed to encode token data: %v", err)
	}

	sessMgr, sessID := newTestSessMgr(map[string]interface{}{
		"method":   "oidc",
		"id_token": encoded,
	})

	h := &Handler{
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		config: config.Config{App: config.AppConfig{RootURL: "http://localhost:7000"}},
		authconfig: map[string]OIDCAuthConfig{
			"test": {
				endSessionEndpoint: "", // no end-session endpoint
				oauth2Config: &oauth2.Config{ClientID: "client-id"},
			},
		},
		sessMgr: sessMgr,
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: sessID})
	rec := httptest.NewRecorder()

	if err := h.HandleLogout(e.NewContext(req, rec)); err != nil {
		t.Fatalf("HandleLogout returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if xRedirect := rec.Header().Get("x-redirect"); xRedirect != "" {
		t.Fatalf("expected no x-redirect header, got %q", xRedirect)
	}
}

// TestHandleLogoutPasswordSessionNoRedirect verifies that a password-based
// session is destroyed without setting x-redirect.
func TestHandleLogoutPasswordSessionNoRedirect(t *testing.T) {
	sessMgr, sessID := newTestSessMgr(map[string]interface{}{
		"method": "password",
	})

	h := &Handler{
		logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		config:     config.Config{App: config.AppConfig{RootURL: "http://localhost:7000"}},
		authconfig: make(map[string]OIDCAuthConfig),
		sessMgr:    sessMgr,
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: sessID})
	rec := httptest.NewRecorder()

	if err := h.HandleLogout(e.NewContext(req, rec)); err != nil {
		t.Fatalf("HandleLogout returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if xRedirect := rec.Header().Get("x-redirect"); xRedirect != "" {
		t.Fatalf("expected no x-redirect header, got %q", xRedirect)
	}
}

// TestHandleLogoutNoSessionReturns200 verifies that HandleLogout returns 200
// even when there is no active session.
func TestHandleLogoutNoSessionReturns200(t *testing.T) {
	sessMgr, _ := newTestSessMgr(nil)

	h := &Handler{
		logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		config:     config.Config{},
		authconfig: make(map[string]OIDCAuthConfig),
		sessMgr:    sessMgr,
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	// No session cookie — Acquire will fail with ErrInvalidSession.
	rec := httptest.NewRecorder()

	if err := h.HandleLogout(e.NewContext(req, rec)); err != nil {
		t.Fatalf("HandleLogout returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}
