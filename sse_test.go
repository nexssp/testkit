package testkit_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nexssp/kernel/action"
	"github.com/nexssp/testkit"
	"github.com/nexssp/transport/thttp"
)

func TestSSE_StreamCaptureAndWaitFor(t *testing.T) {
	t.Parallel()

	broadcaster := thttp.NewBroadcaster(10)

	// Stream Action
	streamAct := action.New[any, any]("events.stream", nil).
		Route(thttp.Stream("/events/live").WithBroadcaster(broadcaster)).
		Build()

	// Trigger Action that publishes to the broadcaster
	triggerAct := action.New("events.trigger", func(_ context.Context, req struct{ Message string }) (string, error) {
		broadcaster.Publish("/events/live", []byte(`{"text":"`+req.Message+`"}`))
		return "published", nil
	}).Route(thttp.POST("/events/trigger")).Build()

	suite := testkit.New(t, streamAct, triggerAct)

	// 1. Connect SSE listener (synchronizes on initial 'connected' handshake)
	stream := suite.ListenSSE("/events/live")
	defer stream.Close()

	// 2. Fire trigger to emit message
	suite.POST("/events/trigger", map[string]string{"Message": "order_confirmed"}).
		Do().
		ExpectCreated()

	// 3. Block until event payload arrives
	evt := stream.WaitFor(t, "message", 2*time.Second)

	if !strings.Contains(evt.Data, "order_confirmed") {
		t.Fatalf("expected SSE payload containing 'order_confirmed', got: %q", evt.Data)
	}
}

func TestSSE_EndpointHelper(t *testing.T) {
	// Minimal MCP-style SSE handler
	mux := http.NewServeMux()
	mux.HandleFunc("GET /mcp/sse", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Error("streaming unsupported")
			return
		}

		_, _ = fmt.Fprintf(w, "event: endpoint\ndata: /mcp/message?sessionId=abc123\n\n")
		flusher.Flush()

		<-r.Context().Done() // keep connection open until cleanup
	})

	suite := testkit.NewWithHandler(t, mux)
	stream := suite.ListenSSE("/mcp/sse")
	defer stream.Close()

	endpoint := stream.Endpoint(t, 2*time.Second)

	if !strings.Contains(endpoint, "/mcp/message?sessionId=") {
		t.Fatalf("unexpected endpoint: %s", endpoint)
	}
}

func TestSSE_ListenSSEWithRequest_Post(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /rpc/stream", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if got := r.Header.Get("Accept"); got != "text/event-stream" {
			http.Error(w, "bad accept", http.StatusBadRequest)
			return
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "event: message\ndata: {\"status\":\"ok\"}\n\n")
		flusher.Flush()

		<-r.Context().Done()
	})

	suite := testkit.NewWithHandler(t, mux)

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/rpc/stream",
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	stream := suite.ListenSSEWithRequest(t, req)
	defer stream.Close()

	evt := stream.WaitFor(t, "message", 2*time.Second)

	if !strings.Contains(evt.Data, `"status":"ok"`) {
		t.Fatalf("unexpected SSE data: %q", evt.Data)
	}
}
