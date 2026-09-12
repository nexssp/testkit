package testkit

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

const defaultEventuallyInterval = 10 * time.Millisecond

// Eventually repeatedly evaluates condition until it returns true or timeout
// expires. The condition is evaluated immediately before waiting for the first
// interval. It runs synchronously on the caller's goroutine and Eventually
// starts no goroutines.
func Eventually(t testing.TB, timeout, interval time.Duration, condition func() bool) {
	t.Helper()

	if condition == nil {
		t.Fatal("testkit.Eventually: nil condition")
	}

	if condition() {
		return
	}

	if timeout <= 0 {
		t.Fatalf("testkit.Eventually: condition was false (timeout=%s)", timeout)
		return
	}

	interval = normalizePollInterval(timeout, interval)
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if condition() {
				return
			}
		case <-timer.C:
			t.Fatalf("testkit.Eventually: condition was false after %s", timeout)
			return
		}
	}
}

// WaitForJSON repeats request until it returns a decodable JSON value accepted
// by ready, or timeout expires. request must build a fresh request on every
// call. Transient request and JSON-decoding errors are retried; the last error
// is included in the timeout failure when one is available.
//
// This is intended for eventually consistent HTTP APIs. For ordinary one-shot
// assertions, use suite.GET(...).Do().Into(...) instead.
func WaitForJSON[T any](
	t testing.TB,
	timeout, interval time.Duration,
	request func() (*Response, error),
	ready func(T) bool,
) T {
	t.Helper()

	if request == nil {
		t.Fatal("testkit.WaitForJSON: nil request")
	}
	if ready == nil {
		t.Fatal("testkit.WaitForJSON: nil ready predicate")
	}

	var (
		lastErr error
		value   T
	)

	try := func() bool {
		response, err := request()
		if err != nil {
			lastErr = err
			return false
		}
		if response == nil {
			lastErr = fmt.Errorf("nil HTTP response")
			return false
		}
		if err := json.Unmarshal(response.body, &value); err != nil {
			lastErr = err
			return false
		}
		return ready(value)
	}

	if try() {
		return value
	}
	if timeout <= 0 {
		failWaitForJSON(t, timeout, lastErr)
		return value
	}

	interval = normalizePollInterval(timeout, interval)
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if try() {
				return value
			}
		case <-timer.C:
			failWaitForJSON(t, timeout, lastErr)
			return value
		}
	}
}

func failWaitForJSON(t testing.TB, timeout time.Duration, lastErr error) {
	t.Helper()
	if lastErr != nil {
		t.Fatalf("testkit.WaitForJSON: condition was not satisfied after %s; last error: %v", timeout, lastErr)
		return
	}
	t.Fatalf("testkit.WaitForJSON: condition was not satisfied after %s", timeout)
}

func normalizePollInterval(timeout, interval time.Duration) time.Duration {
	if interval <= 0 {
		interval = defaultEventuallyInterval
	}
	if interval > timeout {
		interval = timeout
	}
	return interval
}
