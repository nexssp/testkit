package testkit_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nexssp/kernel/action"
	"github.com/nexssp/testkit"
)

func TestConcurrency_ThunderingHerdBarrier(t *testing.T) {
	t.Parallel()

	var activeWorkers atomic.Int32
	var peakConcurrency atomic.Int32
	var totalExecutions atomic.Int32

	act := action.New("barrier.test", func(_ context.Context, req int) (int, error) {
		totalExecutions.Add(1)
		cur := activeWorkers.Add(1)

		for {
			old := peakConcurrency.Load()
			if cur <= old || peakConcurrency.CompareAndSwap(old, cur) {
				break
			}
		}

		time.Sleep(10 * time.Millisecond) // Hold in critical section
		activeWorkers.Add(-1)
		return req * 2, nil
	}).Build()

	const workers = 50

	// Launch 50 concurrent requests behind the synchronized start barrier
	testkit.Simulate(t, act, 10, workers, func(t testing.TB, res int, err error) {
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if res != 20 {
			t.Errorf("expected 20, got %d", res)
		}
	})

	if totalExecutions.Load() != workers {
		t.Fatalf("expected %d executions, got %d", workers, totalExecutions.Load())
	}

	// Verify that the barrier forced true concurrent execution
	if peakConcurrency.Load() < 10 {
		t.Fatalf("barrier failed: peak concurrency was only %d (expected >= 10)", peakConcurrency.Load())
	}
}
