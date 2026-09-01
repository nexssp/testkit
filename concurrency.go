package testkit

import (
	"context"
	"sync"
	"testing"

	"github.com/nexssp/kernel/action"
)

// Simulate executes an action concurrently across N workers using a synchronized
// barrier to verify thread-safety, race conditions, and singleflight deduplication.
func Simulate[Req, Res any](
	t testing.TB,
	act *action.BuiltAction[Req, Res],
	req Req,
	concurrency int,
	assertFn func(t testing.TB, res Res, err error),
) {
	t.Helper()
	var wg sync.WaitGroup
	startBarrier := make(chan struct{})

	for range concurrency {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startBarrier

			res, err := act.Do(context.Background(), req)
			assertFn(t, res, err)
		}()
	}

	close(startBarrier)
	wg.Wait()
}
