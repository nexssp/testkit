package testkit

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/nexssp/kernel/xerr"
)

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
	if err := json.Unmarshal(c.body, &root); err != nil {
		c.t.Fatalf("testkit: invalid JSON body: %v", err)
		return c
	}

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
	if errRes.Error != string(kind) {
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
