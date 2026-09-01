package testkit

import (
	"bufio"
	"bytes"
	"context"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

type SSEEvent struct {
	Event string
	Data  string
}

type StreamCapture struct {
	events chan SSEEvent
	cancel context.CancelFunc
}

// ListenSSE connects to an SSE endpoint using GET.
func (s *Suite) ListenSSE(path string) *StreamCapture {
	s.T.Helper()

	return s.listenSSE(s.T, func(ctx context.Context) *http.Request {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL+path, http.NoBody)
		if err != nil {
			s.T.Fatalf("ListenSSE: failed to build request: %v", err)
		}
		req.Header.Set("Accept", "text/event-stream")
		return req
	})
}

// ListenSSEWithRequest connects to an SSE endpoint using a custom HTTP request.
// This supports POST + SSE patterns such as MCP Streamable HTTP and A2A.
func (s *Suite) ListenSSEWithRequest(t testing.TB, req *http.Request) *StreamCapture {
	t.Helper()

	return s.listenSSE(t, func(ctx context.Context) *http.Request {
		cloned := req.Clone(ctx)

		// httptest.NewRequest creates a server-side request.
		// http.Client.Do requires RequestURI to be empty.
		cloned.RequestURI = ""

		if req.URL != nil {
			u := *req.URL
			cloned.URL = &u
		} else {
			cloned.URL = &url.URL{}
		}

		s.setBaseURL(t, cloned)

		if cloned.Header.Get("Accept") == "" {
			cloned.Header.Set("Accept", "text/event-stream")
		}

		return cloned
	})
}

func (s *Suite) setBaseURL(t testing.TB, req *http.Request) {
	t.Helper()

	base, err := url.Parse(s.baseURL)
	if err != nil {
		t.Fatalf("ListenSSE: invalid base URL %q: %v", s.baseURL, err)
	}

	req.URL.Scheme = base.Scheme
	req.URL.Host = base.Host

	if req.URL.Path == "" {
		req.URL.Path = "/"
	}
	if !strings.HasPrefix(req.URL.Path, "/") {
		req.URL.Path = "/" + req.URL.Path
	}
}

func (s *Suite) listenSSE(t testing.TB, buildReq func(ctx context.Context) *http.Request) *StreamCapture {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	streamCap := &StreamCapture{
		events: make(chan SSEEvent, 100),
		cancel: cancel,
	}
	t.Cleanup(streamCap.Close)

	req := buildReq(ctx)

	s.mu.RLock()
	for k, v := range s.headers {
		req.Header.Set(k, v)
	}
	for _, c := range s.cookies {
		req.AddCookie(c)
	}
	s.mu.RUnlock()

	ready := make(chan struct{})
	var readyOnce sync.Once

	go func() {
		resp, err := s.client.Do(req)
		readyOnce.Do(func() { close(ready) })

		if err != nil {
			return
		}
		defer resp.Body.Close()

		reader := bufio.NewReader(resp.Body)
		var currentEvent string
		var currentData bytes.Buffer

		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimSpace(line)

			if line == "" {
				if currentEvent != "" || currentData.Len() > 0 {
					evt := SSEEvent{Event: currentEvent, Data: currentData.String()}
					select {
					case streamCap.events <- evt:
					default:
					}
					currentEvent = ""
					currentData.Reset()
				}
				continue
			}

			if after, ok := strings.CutPrefix(line, "event:"); ok {
				currentEvent = strings.TrimSpace(after)
			} else if after, ok := strings.CutPrefix(line, "data:"); ok {
				currentData.WriteString(strings.TrimSpace(after))
			}
		}
	}()

	select {
	case <-ready:
	case <-time.After(3 * time.Second):
		streamCap.Close()
		t.Fatalf("ListenSSE: timed out waiting for SSE connection on %s", req.URL.Path)
	}

	return streamCap
}

func (sc *StreamCapture) WaitFor(t testing.TB, eventName string, timeout time.Duration) SSEEvent {
	t.Helper()
	deadline := time.After(timeout)

	for {
		select {
		case evt := <-sc.events:
			if eventName == "" || evt.Event == eventName {
				return evt
			}
		case <-deadline:
			t.Fatalf("ListenSSE: timed out after %v waiting for event %q", timeout, eventName)
			return SSEEvent{}
		}
	}
}

func (sc *StreamCapture) WaitForData(t testing.TB, substr string, timeout time.Duration) SSEEvent {
	t.Helper()
	deadline := time.After(timeout)

	for {
		select {
		case evt := <-sc.events:
			if strings.Contains(evt.Data, substr) {
				return evt
			}
		case <-deadline:
			t.Fatalf("ListenSSE: timed out after %v waiting for data containing %q", timeout, substr)
			return SSEEvent{}
		}
	}
}

// Endpoint conveniently extracts an MCP SSE endpoint event.
func (sc *StreamCapture) Endpoint(t testing.TB, timeout time.Duration) string {
	t.Helper()
	evt := sc.WaitFor(t, "endpoint", timeout)
	return strings.TrimSpace(evt.Data)
}

func (sc *StreamCapture) Close() {
	sc.cancel()
}
