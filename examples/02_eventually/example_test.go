package eventually

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/nexssp/kernel/xtest"
)

func TestEventually_WaitsForAsyncWork(t *testing.T) {
	var done atomic.Bool

	go func() {
		time.Sleep(30 * time.Millisecond)
		done.Store(true)
	}()

	start := time.Now()

	xtest.Eventually(t, 2*time.Second, done.Load)

	elapsed := time.Since(start)
	if elapsed > 500*time.Millisecond {
		t.Fatalf("Eventually took %s — polling too slowly or timeout is wrong", elapsed)
	}
}
