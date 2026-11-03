package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
)

type call struct {
	method      string
	path        string
	query       url.Values
	body        io.Reader
	contentType string
	header      http.Header
}

type dispatcher struct {
	h http.Handler
}

func (d *dispatcher) do(ctx context.Context, auth, remoteAddr string, c call) (int, []byte, error) {
	var buf bytes.Buffer
	code, err := d.stream(ctx, auth, remoteAddr, c, &buf)
	return code, buf.Bytes(), err
}

func (d *dispatcher) stream(ctx context.Context, auth, remoteAddr string, c call, w io.Writer) (int, error) {
	target := c.path
	if len(c.query) > 0 {
		target += "?" + c.query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, c.method, target, c.body)
	if err != nil {
		return 0, err
	}
	req.RemoteAddr = remoteAddr
	for k, v := range c.header {
		req.Header[k] = v
	}
	req.Header.Set("Authorization", auth)
	if c.contentType != "" {
		req.Header.Set("Content-Type", c.contentType)
	}

	rec := &recorder{header: make(http.Header), w: w}
	d.h.ServeHTTP(rec, req)
	if rec.code == 0 {
		rec.code = http.StatusOK
	}
	return rec.code, nil
}

type recorder struct {
	header http.Header
	code   int
	w      io.Writer
}

func (r *recorder) Header() http.Header {
	return r.header
}

func (r *recorder) WriteHeader(code int) {
	if r.code == 0 {
		r.code = code
	}
}

func (r *recorder) Write(b []byte) (int, error) {
	if r.code == 0 {
		r.code = http.StatusOK
	}
	return r.w.Write(b)
}

type api struct {
	d          *dispatcher
	auth       string
	remoteAddr string
	status     int
}

func (a *api) stream(ctx context.Context, c call, w io.Writer) (int, error) {
	code, err := a.d.stream(ctx, a.auth, a.remoteAddr, c, w)
	if err != nil {
		return 0, &jsonrpc.Error{Code: jsonrpc.CodeInternalError, Message: fmt.Sprintf("could not dispatch request: %v", err)}
	}
	a.status = code
	return code, nil
}

func (a *api) do(ctx context.Context, c call) ([]byte, error) {
	var buf bytes.Buffer
	code, err := a.stream(ctx, c, &buf)
	if err != nil {
		return nil, err
	}
	if code < 200 || code > 299 {
		return nil, responseError(code, buf.Bytes())
	}
	return buf.Bytes(), nil
}

func (a *api) get(ctx context.Context, path string, query url.Values) ([]byte, error) {
	return a.do(ctx, call{method: http.MethodGet, path: path, query: query})
}

func (a *api) send(ctx context.Context, method, path string, body any) ([]byte, error) {
	c := call{method: method, path: path}
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, &jsonrpc.Error{Code: jsonrpc.CodeInternalError, Message: fmt.Sprintf("could not encode request: %v", err)}
		}
		c.body = bytes.NewReader(b)
		c.contentType = "application/json"
	}
	return a.do(ctx, c)
}

func responseError(code int, body []byte) error {
	var resp struct {
		Error   string          `json:"error"`
		Code    string          `json:"code"`
		Details json.RawMessage `json:"details"`
	}
	msg := http.StatusText(code)
	if json.Unmarshal(body, &resp) == nil {
		switch {
		case resp.Error != "":
			msg = resp.Error
		case resp.Code != "":
			msg = resp.Code
		}
		if len(resp.Details) > 0 && string(resp.Details) != "null" {
			msg += ": " + string(resp.Details)
		}
	}
	return errors.New(msg)
}

func apiPath(segments ...string) string {
	escaped := make([]string, len(segments))
	for i, s := range segments {
		escaped[i] = url.PathEscape(s)
	}
	return "/api/v1/" + strings.Join(escaped, "/")
}

func result(body []byte) any {
	if len(bytes.TrimSpace(body)) == 0 {
		return map[string]any{"ok": true}
	}
	return json.RawMessage(body)
}

func withFields(body []byte, fields map[string]any) (map[string]any, error) {
	out := make(map[string]any)
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("could not decode response: %w", err)
	}
	for k, v := range fields {
		out[k] = v
	}
	return out, nil
}

func pageQuery(page int, filter string) url.Values {
	q := url.Values{}
	if page > 0 {
		q.Set("page", fmt.Sprint(page))
	}
	if filter != "" {
		q.Set("filter", filter)
	}
	return q
}
