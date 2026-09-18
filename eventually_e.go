package testkit

import (
	"testing"
	"time"
)

// EventuallyE repeats fn until it returns nil, or timeout expires.
//
// Unlike Eventually, fn returns an error, and EventuallyE returns the
// last error observed. This is the right shape for asserting on
// operations that must complete successfully (commit, flush, connect).
func EventuallyE(t testing.TB, timeout, interval time.Duration, fn func() error) error {
	t.Helper()

	if fn == nil {
		t.Fatal("testkit.EventuallyE: nil fn")
		return nil
	}

	deadline := time.Now().Add(timeout)

	lastErr := fn()
	if lastErr == nil {
		return nil
	}
	if timeout <= 0 {
		return lastErr
	}

	interval = normalizePollInterval(timeout, interval)
	timer := time.NewTimer(time.Until(deadline))
	defer timer.Stop()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if lastErr = fn(); lastErr == nil {
				return nil
			}
		case <-timer.C:
			return lastErr
		}
	}
}
