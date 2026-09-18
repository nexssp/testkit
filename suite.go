package testkit

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nexssp/kernel/action"
	"github.com/nexssp/kernel/xctx"
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
	s.cookies = append(s.cookies, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
	return s
}

func (s *Suite) ResetHeaders() *Suite {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.headers = make(map[string]string)
	s.cookies = nil
	return s
}
