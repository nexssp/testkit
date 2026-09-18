// Package simulate demonstrates the concurrency barrier.
//
// Simulate launches N goroutines that all wait behind a channel, then
// releases them at the same instant. The handler sees true concurrent
// load, not "one goroutine started 10µs before the others."
//
// This is the pattern that catches map races, shared caches missing
// locks, and singleflight bugs — the classes of defects that a
// sequential test will never find.
package simulate

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/nexssp/kernel/action"
	"github.com/nexssp/testkit"
)

func TestSimulate_ThunderingHerd(t *testing.T) {
	var (
		active, peak, total atomic.Int32
		arrived             atomic.Int32
	)

	const workers = 64

	gate := make(chan struct{})

	act := action.New("handler", func(_ context.Context, n int) (int, error) {
		total.Add(1)
		cur := active.Add(1)
		for {
			old := peak.Load()
			if cur <= old || peak.CompareAndSwap(old, cur) {
				break
			}
		}

		// Block until every worker has entered the handler. Only then
		// do we let them proceed. This makes "peak concurrency" a
		// property of the test, not of the Go scheduler.
		if arrived.Add(1) == workers {
			close(gate)
		}
		<-gate

		active.Add(-1)
		return n * 2, nil
	}).Build()

	testkit.Simulate(t, act, 21, workers, func(t testing.TB, res int, err error) {
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if res != 42 {
			t.Errorf("expected 42, got %d", res)
		}
	})

	if total.Load() != workers {
		t.Fatalf("expected %d executions, got %d", workers, total.Load())
	}
	if peak.Load() != workers {
		t.Fatalf("expected all %d workers in flight, got peak %d", workers, peak.Load())
	}
}
