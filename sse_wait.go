package testkit

import (
	"strings"
	"testing"
	"time"
)

// WaitForSSE waits for the first buffered SSE event accepted by match. Events
// that do not match are discarded, just like StreamCapture.WaitFor and
// WaitForData. A nil match predicate is rejected to avoid an accidental
// match-all wait; use WaitFor with an empty event name when matching any event.
func (sc *StreamCapture) WaitForSSE(t testing.TB, timeout time.Duration, match func(SSEEvent) bool) SSEEvent {
	t.Helper()

	if sc == nil {
		t.Fatal("testkit.WaitForSSE: nil stream capture")
		return SSEEvent{}
	}
	if match == nil {
		t.Fatal("testkit.WaitForSSE: nil match predicate")
		return SSEEvent{}
	}

	deadline := time.NewTimer(timeout)
	defer deadline.Stop()

	for {
		select {
		case event := <-sc.events:
			if match(event) {
				return event
			}
		case <-deadline.C:
			t.Fatalf("testkit.WaitForSSE: timed out after %s waiting for a matching event", timeout)
			return SSEEvent{}
		}
	}
}

// WaitForSSEData waits for an SSE event whose data contains substr. It is the
// predicate equivalent of StreamCapture.WaitForData and is useful when a test
// needs the event name as well as a data substring.
func (sc *StreamCapture) WaitForSSEData(t testing.TB, substr string, timeout time.Duration) SSEEvent {
	t.Helper()
	return sc.WaitForSSE(t, timeout, func(event SSEEvent) bool {
		return strings.Contains(event.Data, substr)
	})
}
