package testkit

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/nexssp/kernel/xtest"
)

// errNotReady is returned to EventuallyEveryE when a poll produced a
// decodable body that did not yet satisfy the caller's ready predicate.
// It exists purely so the retry loop has something non-nil to observe;
// the message is what a caller sees if the test times out.
var errNotReady = errors.New("testkit: response decoded but not ready")

// WaitForJSON polls request until the response body decodes as T and
// satisfies ready, or until timeout elapses. It returns the last
// successfully-decoded value.
//
// All polling and timeout semantics are delegated to
// kernel/xtest.EventuallyEveryE, so HTTP polling behaves exactly like
// every other Eventually assertion in the kernel. This function adds
// only the decode-then-check step.
//
//   - nil ready accepts any decodable body.
//   - timeout <= 0 performs a single synchronous attempt (matching
//     EventuallyEveryE's contract).
//   - a non-positive interval is normalized by the kernel poll loop.
//
// On timeout the test is failed via t, and the last observed error
// (transport error, JSON error, or errNotReady) is included in the
// failure message.
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

	var last T

	err := xtest.EventuallyEveryE(t, timeout, interval, func() error {
		resp, err := request()
		if err != nil {
			return err
		}
		if resp == nil {
			return errors.New("testkit: request returned (nil, nil)")
		}

		var got T
		if err := json.Unmarshal(resp.Body(), &got); err != nil {
			return err
		}
		if ready != nil && !ready(got) {
			return errNotReady
		}

		last = got
		return nil
	})
	if err != nil {
		t.Fatalf("testkit.WaitForJSON: %v", err)
	}

	return last
}
