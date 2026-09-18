// Package eventually demonstrates polling for async completion.
//
// The classic "wait for the cache janitor to evict the key" test — the
// one where a naive test sleeps 200ms and hopes the scheduler is kind.
// Eventually turns the hope into a guarantee: it returns as soon as the
// condition is true, and fails only when the timeout genuinely expires.
package eventually

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/nexssp/testkit"
)

func TestEventually_WaitsForAsyncWork(t *testing.T) {
	var done atomic.Bool

	go func() {
		time.Sleep(30 * time.Millisecond)
		done.Store(true)
	}()

	start := time.Now()

	testkit.Eventually(t, 2*time.Second, 5*time.Millisecond, done.Load)

	elapsed := time.Since(start)
	if elapsed > 500*time.Millisecond {
		t.Fatalf("Eventually took %s — it is polling too slowly or the timeout is wrong", elapsed)
	}
}
