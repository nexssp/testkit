// Package testkit provides deterministic, fluent test harnesses for both
// nexssp actions and standard Go http.Handler implementations.
package testkit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nexssp/kernel/action"
	"github.com/nexssp/kernel/xctx"
	"github.com/nexssp/kernel/xerr"
	"github.com/nexssp/testkit/chaos"
	"github.com/nexssp/transport/thttp"
)

const (
	HeaderTenantID = "X-Tenant-ID"
)

// Suite wraps an in-memory or live HTTP server with fluent test assertions.
type Suite struct {
	T       testing.TB
	Server  *httptest.Server
	baseURL string
	client  *http.Client
	mu      sync.RWMutex
	headers map[string]string
	cookies []*http.Cookie
}

// New flattens nexssp actions and mounts them onto an in-memory transport.
func New(t testing.TB, providers ...any) *Suite {
	t.Helper()
	var allActions []action.AnyAction

	var flatten func(items ...any)
	flatten = func(items ...any) {
		for _, p := range items {
			if p == nil {
				continue
			}
			switch v := p.(type) {
			case action.AnyAction:
				allActions = append(allActions, v)
			case []action.AnyAction:
				for _, act := range v {
					if act != nil {
						allActions = append(allActions, act)
					}
				}
			case []any:
				flatten(v...)
			default:
				t.Fatalf("testkit.New: unsupported type %T. Must be action.AnyAction or []action.AnyAction", p)
			}
		}
	}
	flatten(providers...)

	var hooks []action.AnyHook
	var routes []action.AnyAction
	for _, act := range allActions {
		if len(act.GetBindings()) == 0 {
			hooks = append(hooks, act.GetAnyHooks()...)
		} else {
			routes = append(routes, act)
		}
	}

	if len(hooks) > 0 {
		for _, act := range routes {
			for _, h := range hooks {
				act.AddAnyHook(h)
			}
		}
	}

	tr := thttp.New(":0")
	tr.Mount(routes)

	handler := contextBridgeMiddleware(tr.Handler())
	return NewWithHandler(t, handler)
}

// NewWithHandler initializes a test suite against ANY standard Go http.Handler.
func NewWithHandler(t testing.TB, h http.Handler) *Suite {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	return &Suite{
		T:       t,
		Server:  srv,
		baseURL: srv.URL,
		client:  srv.Client(),
		headers: make(map[string]string),
	}
}

// NewE2E initializes the suite against a live, external URL.
func NewE2E(t testing.TB, baseURL string) *Suite {
	t.Helper()
	return &Suite{
		T:       t,
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 30 * time.Second},
		headers: make(map[string]string),
	}
}

func contextBridgeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if tid := r.Header.Get(HeaderTenantID); tid != "" {
			ctx = xctx.WithTenantID(ctx, tid)
		}
		if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
			ctx = xctx.WithUserID(ctx, "test_user")
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ── Global Configuration ──────────────────────────────────────────────────────

func (s *Suite) WithGlobalBearerToken(token string) *Suite {
	return s.WithGlobalHeader("Authorization", "Bearer "+token)
}

func (s *Suite) WithGlobalHeader(key, value string) *Suite {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.headers[key] = value
	return s
}

func (s *Suite) WithCookie(name, value string) *Suite {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cookies = append(s.cookies, &http.Cookie{Name: name, Value: value})
	return s
}

func (s *Suite) ResetHeaders() *Suite {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.headers = make(map[string]string)
	s.cookies = nil
	return s
}

// ── HTTP Verb Methods ─────────────────────────────────────────────────────────

func (s *Suite) GET(path string, body ...any) *Request { return s.verb(http.MethodGet, path, body...) }
func (s *Suite) POST(path string, body ...any) *Request {
	return s.verb(http.MethodPost, path, body...)
}
func (s *Suite) PUT(path string, body ...any) *Request { return s.verb(http.MethodPut, path, body...) }
func (s *Suite) PATCH(path string, body ...any) *Request {
	return s.verb(http.MethodPatch, path, body...)
}
func (s *Suite) DELETE(path string, body ...any) *Request {
	return s.verb(http.MethodDelete, path, body...)
}

func (s *Suite) verb(method, path string, body ...any) *Request {
	r := s.Request(method, path)
	if len(body) > 0 && body[0] != nil {
		r.WithJSON(body[0])
	}
	return r
}

func (s *Suite) Request(method, path string) *Request {
	return &Request{
		s:       s,
		method:  method,
		path:    path,
		headers: make(map[string]string),
		ctx:     context.Background(),
	}
}

// ── Request Builder ───────────────────────────────────────────────────────────

type Request struct {
	s       *Suite
	method  string
	path    string
	body    []byte
	headers map[string]string
	cookies []*http.Cookie
	ctx     context.Context
	err     error // Delayed builder error to protect goroutines from fatal calls
}

func (r *Request) WithQuery(key, value string) *Request {
	q := r.parseQuery()
	q.Set(key, value)
	r.path = strings.SplitN(r.path, "?", 2)[0] + "?" + q.Encode()
	return r
}

func (r *Request) WithQueries(params map[string]string) *Request {
	q := r.parseQuery()
	for k, v := range params {
		q.Set(k, v)
	}
	r.path = strings.SplitN(r.path, "?", 2)[0] + "?" + q.Encode()
	return r
}

func (r *Request) WithJSON(v any) *Request {
	if r.err != nil {
		return r
	}
	b, err := json.Marshal(v)
	if err != nil {
		r.err = fmt.Errorf("testkit: WithJSON marshal failed: %w", err)
		return r
	}
	r.body = b
	r.headers["Content-Type"] = "application/json"
	return r
}

func (r *Request) WithMultipartFile(fieldName, filename string, content []byte) *Request {
	if r.err != nil {
		return r
	}
	buf := new(bytes.Buffer)
	w := multipart.NewWriter(buf)
	fw, err := w.CreateFormFile(fieldName, filename)
	if err != nil {
		r.err = fmt.Errorf("testkit: WithMultipartFile CreateFormFile failed: %w", err)
		return r
	}
	if _, err := fw.Write(content); err != nil {
		r.err = fmt.Errorf("testkit: WithMultipartFile write failed: %w", err)
		return r
	}
	_ = w.Close()
	r.body = buf.Bytes()
	r.headers["Content-Type"] = w.FormDataContentType()
	return r
}

func (r *Request) WithContext(ctx context.Context) *Request {
	r.ctx = ctx
	return r
}

func (r *Request) WithTenant(tenantID string) *Request {
	r.headers[HeaderTenantID] = tenantID
	return r
}

func (r *Request) WithHeader(k, v string) *Request {
	r.headers[k] = v
	return r
}

func (r *Request) WithBearerToken(token string) *Request {
	return r.WithHeader("Authorization", "Bearer "+token)
}

func (r *Request) WithForm(data map[string]string) *Request {
	values := url.Values{}
	for k, v := range data {
		values.Set(k, v)
	}
	r.body = []byte(values.Encode())
	r.headers["Content-Type"] = "application/x-www-form-urlencoded"
	return r
}

func (r *Request) WithCookie(name, value string) *Request {
	r.cookies = append(r.cookies, &http.Cookie{Name: name, Value: value})
	return r
}

// DoContext executes the HTTP request with the provided context and returns an error without calling t.Fatalf (goroutine safe).
func (r *Request) DoContext(ctx context.Context) (*Response, error) {
	if r.err != nil {
		return nil, r.err
	}

	reqURL := r.s.baseURL + r.path

	var bodyReader io.Reader
	if len(r.body) > 0 {
		bodyReader = bytes.NewReader(r.body)
	}

	req, err := http.NewRequestWithContext(ctx, r.method, reqURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("testkit: request build failed: %w", err)
	}

	r.s.mu.RLock()
	for k, v := range r.s.headers {
		req.Header.Set(k, v)
	}
	for _, c := range r.s.cookies {
		req.AddCookie(c)
	}
	r.s.mu.RUnlock()

	for k, v := range r.headers {
		req.Header.Set(k, v)
	}
	for _, c := range r.cookies {
		req.AddCookie(c)
	}

	resp, err := r.s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("testkit: %s %s failed: %w", r.method, r.path, err)
	}
	raw, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("testkit: reading response body failed: %w", err)
	}

	return &Response{t: r.s.T, resp: resp, body: raw}, nil
}

// DoE executes the HTTP request using the request's configured context.
func (r *Request) DoE() (*Response, error) {
	return r.DoContext(r.ctx)
}

// Do executes the request and immediately fails the test if an error occurs.
func (r *Request) Do() *Response {
	r.s.T.Helper()
	res, err := r.DoE()
	if err != nil {
		r.s.T.Fatalf("%v", err)
		return nil
	}
	return res
}

func (r *Request) parseQuery() url.Values {
	if idx := strings.Index(r.path, "?"); idx >= 0 {
		q, _ := url.ParseQuery(r.path[idx+1:])
		return q
	}
	return make(url.Values)
}

// ── Response Fluent Assertions ────────────────────────────────────────────────

type Response struct {
	t    testing.TB
	resp *http.Response
	body []byte
}

func (c *Response) Status() int            { return c.resp.StatusCode }
func (c *Response) Body() []byte           { return c.body }
func (c *Response) BodyString() string     { return string(c.body) }
func (c *Response) Header(k string) string { return c.resp.Header.Get(k) }

func (c *Response) ExpectStatus(code int) *Response {
	c.t.Helper()
	if c.resp.StatusCode != code {
		c.t.Fatalf("expected status %d, got %d\nBody: %s", code, c.resp.StatusCode, c.body)
	}
	return c
}

func (c *Response) ExpectSuccess() *Response {
	c.t.Helper()
	if c.resp.StatusCode < 200 || c.resp.StatusCode >= 300 {
		c.t.Fatalf("expected 2xx success, got %d\nBody: %s", c.resp.StatusCode, c.body)
	}
	return c
}

func (c *Response) ExpectOK() *Response           { return c.ExpectStatus(http.StatusOK) }
func (c *Response) ExpectCreated() *Response      { return c.ExpectStatus(http.StatusCreated) }
func (c *Response) ExpectBadRequest() *Response   { return c.ExpectStatus(http.StatusBadRequest) }
func (c *Response) ExpectUnauthorized() *Response { return c.ExpectStatus(http.StatusUnauthorized) }
func (c *Response) ExpectForbidden() *Response    { return c.ExpectStatus(http.StatusForbidden) }
func (c *Response) ExpectNotFound() *Response     { return c.ExpectStatus(http.StatusNotFound) }

func (c *Response) Into(v any) *Response {
	c.t.Helper()
	if err := json.Unmarshal(c.body, v); err != nil {
		c.t.Fatalf("testkit: unmarshal response failed: %v\nBody: %s", err, c.body)
	}
	return c
}

func (c *Response) HasField(path string, expected any) *Response {
	c.t.Helper()
	var root map[string]any
	if err := json.Unmarshal(c.body, &root); err != nil {
		c.t.Fatalf("testkit: body is not a JSON object: %s", c.body)
	}
	got := traverse(root, path)
	if fmt.Sprintf("%v", got) != fmt.Sprintf("%v", expected) {
		c.t.Fatalf("testkit: field %q: expected %v, got %v", path, expected, got)
	}
	return c
}

func (c *Response) ExpectArrayLen(path string, n int) *Response {
	c.t.Helper()
	var root any
	_ = json.Unmarshal(c.body, &root)

	val := root
	if path != "" {
		if m, ok := root.(map[string]any); ok {
			val = traverse(m, path)
		} else {
			c.t.Fatalf("testkit: body is not an object, cannot traverse path %q", path)
		}
	}

	arr, ok := val.([]any)
	if !ok || len(arr) != n {
		c.t.Fatalf("testkit: field %q: expected array length %d, got %v", path, n, val)
	}
	return c
}

func (c *Response) ExpectHeader(key, value string) *Response {
	c.t.Helper()
	if got := c.Header(key); got != value {
		c.t.Fatalf("testkit: header %q: expected %q, got %q", key, value, got)
	}
	return c
}

func (c *Response) ExpectErrorKind(kind xerr.Kind, message string) *Response {
	c.t.Helper()
	var errRes xerr.ErrorResponse
	c.Into(&errRes)
	if string(errRes.Error) != string(kind) {
		c.t.Fatalf("testkit: expected error kind %q, got %q", kind, errRes.Error)
	}
	if message != "" && !strings.Contains(errRes.Message, message) {
		c.t.Fatalf("testkit: expected message to contain %q, got %q", message, errRes.Message)
	}
	return c
}

func (c *Response) ContainsString(substr string) *Response {
	c.t.Helper()
	if !strings.Contains(string(c.body), substr) {
		c.t.Fatalf("testkit: body does not contain %q\nBody: %s", substr, c.body)
	}
	return c
}

func traverse(m map[string]any, path string) any {
	if path == "" {
		return m
	}
	if val, ok := m[path]; ok {
		return val
	}

	parts := strings.Split(path, ".")
	var cur any = m
	for _, p := range parts {
		switch target := cur.(type) {
		case map[string]any:
			cur = target[p]
		case []any:
			idx := 0
			_, _ = fmt.Sscanf(p, "%d", &idx)
			if idx >= 0 && idx < len(target) {
				cur = target[idx]
			} else {
				return nil
			}
		default:
			return nil
		}
	}
	return cur
}

// WithChaos wraps provided actions with chaos fault injection.
func WithChaos(actions []action.AnyAction, cfg chaos.Config) []action.AnyAction {
	hook := chaos.GlobalHook(cfg)
	for _, act := range actions {
		act.AddAnyHook(hook)
	}
	return actions
}
