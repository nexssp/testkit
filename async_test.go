package testkit

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestWaitForJSONRetriesUntilPredicateMatches(t *testing.T) {
	calls := 0

	got := WaitForJSON[struct {
		Ready bool `json:"ready"`
	}](t, time.Second, time.Millisecond, func() (*Response, error) {
		calls++
		ready := calls >= 3
		body, _ := json.Marshal(map[string]bool{"ready": ready})
		return &Response{body: body}, nil
	}, func(value struct {
		Ready bool `json:"ready"`
	},
	) bool {
		return value.Ready
	})

	if !got.Ready {
		t.Fatal("expected ready response")
	}
	if calls != 3 {
		t.Fatalf("request calls = %d, want 3", calls)
	}
}

func TestWaitForJSONRetriesRequestAndDecodeErrors(t *testing.T) {
	calls := 0

	got := WaitForJSON[map[string]string](t, time.Second, time.Millisecond, func() (*Response, error) {
		calls++
		if calls == 1 {
			return nil, os.ErrNotExist
		}
		if calls == 2 {
			return &Response{body: []byte("not-json")}, nil
		}
		return &Response{body: []byte(`{"status":"done"}`)}, nil
	}, func(value map[string]string) bool {
		return value["status"] == "done"
	})

	if got["status"] != "done" {
		t.Fatalf("unexpected result: %#v", got)
	}
	if calls != 3 {
		t.Fatalf("request calls = %d, want 3", calls)
	}
}

func TestWaitForSSEMatchesStructuredPredicate(t *testing.T) {
	capture := &StreamCapture{events: make(chan SSEEvent, 3)}
	capture.events <- SSEEvent{Event: "status", Data: `{"state":"running"}`}
	capture.events <- SSEEvent{Event: "docker", Data: `{"command":"printf jumalu"}`}

	got := capture.WaitForSSE(t, time.Second, func(event SSEEvent) bool {
		return event.Event == "docker" && strings.Contains(event.Data, "jumalu")
	})

	if got.Event != "docker" {
		t.Fatalf("event = %q, want docker", got.Event)
	}
}

func TestWaitForSSEDataMatchesEventPayload(t *testing.T) {
	capture := &StreamCapture{events: make(chan SSEEvent, 1)}
	capture.events <- SSEEvent{Event: "message", Data: "goal accepted"}

	got := capture.WaitForSSEData(t, "goal accepted", time.Second)
	if got.Event != "message" {
		t.Fatalf("event = %q, want message", got.Event)
	}
}

func TestWaitForJSONRejectsNilRequest(t *testing.T) {
	if os.Getenv("TESTKIT_WAIT_JSON_FAIL_NIL_REQUEST") == "1" {
		WaitForJSON[struct{}](t, time.Second, time.Millisecond, nil, func(struct{}) bool { return true })
		return
	}

	assertSubprocessFailure(t, "TestWaitForJSONRejectsNilRequest", "nil request", "TESTKIT_WAIT_JSON_FAIL_NIL_REQUEST")
}

func TestWaitForSSERejectsNilPredicate(t *testing.T) {
	if os.Getenv("TESTKIT_WAIT_SSE_FAIL_NIL_PREDICATE") == "1" {
		capture := &StreamCapture{events: make(chan SSEEvent)}
		capture.WaitForSSE(t, time.Second, nil)
		return
	}

	assertSubprocessFailure(t, "TestWaitForSSERejectsNilPredicate", "nil match predicate", "TESTKIT_WAIT_SSE_FAIL_NIL_PREDICATE")
}

func assertSubprocessFailure(t *testing.T, testName, want, envName string) {
	t.Helper()
	cmd := testCommand(t, testName)
	cmd.Env = append(os.Environ(), envName+"=1")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected subprocess to fail")
	}
	if !strings.Contains(string(output), want) {
		t.Fatalf("failure output = %q, want %q", output, want)
	}
}

func testCommand(t *testing.T, testName string) *exec.Cmd {
	t.Helper()
	//nolint:gosec // G204: testName is always a compile-time literal at the call site, not user input.
	return exec.CommandContext(t.Context(), os.Args[0], "-test.run=^"+testName+"$", "-test.v")
}
