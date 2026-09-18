package testkit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
)

// ── HTTP Verb Methods on Suite ────────────────────────────────────────────────
//
// These live in request.go because their only purpose is to construct a
// *Request. A reader looking for "how do I send a POST?" finds both the
// verb shorthand and the request builder in the same file.

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
	r.cookies = append(r.cookies, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
	return r
}

// DoContext executes the HTTP request with the provided context and returns an
// error without calling t.Fatalf (goroutine safe).
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
		q, err := url.ParseQuery(r.path[idx+1:])
		if err != nil {
			return make(url.Values)
		}
		return q
	}
	return make(url.Values)
}
