package testkit

import (
	"bufio"
	"bytes"
	"context"
	"net/http"
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

// ListenSSE connects to an SSE endpoint, unblocking immediately on initial HTTP response.
func (s *Suite) ListenSSE(path string) *StreamCapture {
	s.T.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	streamCap := &StreamCapture{
		events: make(chan SSEEvent, 100),
		cancel: cancel,
	}
	s.T.Cleanup(streamCap.Close)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL+path, http.NoBody)
	if err != nil {
		cancel()
		s.T.Fatalf("ListenSSE: failed to build request: %v", err)
	}

	req.Header.Set("Accept", "text/event-stream")

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
		// Close ready on ANY response arrival to prevent hangs on error status codes
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
		s.T.Fatalf("ListenSSE: timed out waiting for SSE connection on %s", path)
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

func (sc *StreamCapture) Close() {
	sc.cancel()
}
