// testkit/trace.go
package testkit

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nexssp/kernel/action"
)

// Trace records every action invocation that passes through the hook it
// installs. It is the domainless primitive behind every "did this run,
// in what order, with what input, how many times" assertion.
//
// Trace is safe for concurrent use. Order is the order in which hook
// callbacks observed the invocation, which matches the actual scheduling
// order for sequential code and is a best-effort causal order under
// concurrency (each entry captures a monotonic sequence number).
type Trace struct {
	mu    sync.Mutex
	calls []Call
	seq   uint64
}

// Call is one recorded action invocation.
type Call struct {
	Seq      uint64
	Action   string
	Request  any
	Response any
	Err      error
	Duration time.Duration
	When     time.Time
}

// NewTrace returns an empty trace.
func NewTrace() *Trace { return &Trace{} }

// Hook returns an AnyHook that appends to the trace. Install it on a
// registry or on individual actions with AddAnyHook or .AnyHook().
func (t *Trace) Hook() action.AnyHook {
	return action.AnyHook{
		Before: func(ctx context.Context, _ any, _ *action.Meta) (context.Context, error) {
			return context.WithValue(ctx, traceStartKey{}, time.Now()), nil
		},
		OnExecuted: func(ctx context.Context, req, res any, _ error, meta *action.Meta) {
			t.record(ctx, meta, req, res, nil)
		},
		OnError: func(ctx context.Context, req any, err error, meta *action.Meta) {
			t.record(ctx, meta, req, nil, err)
		},
	}
}

type traceStartKey struct{}

func (t *Trace) record(ctx context.Context, meta *action.Meta, req, res any, err error) {
	if meta == nil {
		return
	}
	start, _ := ctx.Value(traceStartKey{}).(time.Time)

	t.mu.Lock()
	t.seq++
	t.calls = append(t.calls, Call{
		Seq:      t.seq,
		Action:   meta.Name,
		Request:  req,
		Response: res,
		Err:      err,
		Duration: time.Since(start),
		When:     time.Now(),
	})
	t.mu.Unlock()
}

// Calls returns a copy of the recorded calls in order.
func (t *Trace) Calls() []Call {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]Call(nil), t.calls...)
}

// Reset clears the trace. Useful between subtests.
func (t *Trace) Reset() {
	t.mu.Lock()
	t.calls = nil
	t.seq = 0
	t.mu.Unlock()
}

// Names returns just the recorded action names, in order.
func (t *Trace) Names() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]string, len(t.calls))
	for i, c := range t.calls {
		out[i] = c.Action
	}
	return out
}

// Count returns how many times action ran.
func (t *Trace) Count(name string) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	n := 0
	for _, c := range t.calls {
		if c.Action == name {
			n++
		}
	}
	return n
}

// AssertSequence fails the test unless the recorded action names equal
// want exactly (length and order).
func (t *Trace) AssertSequence(tb testing.TB, want ...string) {
	tb.Helper()
	got := t.Names()
	if slices.Equal(got, want) {
		return
	}
	tb.Fatalf("trace sequence mismatch\n  want: %v\n  got:  %v", want, got)
}

// AssertSubsequence fails the test unless want appears as a contiguous
// run inside the trace.
func (t *Trace) AssertSubsequence(tb testing.TB, want ...string) {
	tb.Helper()
	got := t.Names()
	if len(want) == 0 {
		return
	}
	for i := 0; i+len(want) <= len(got); i++ {
		if slices.Equal(got[i:i+len(want)], want) {
			return
		}
	}
	tb.Fatalf("trace does not contain subsequence %v\n  got: %v", want, got)
}

// AssertCalled fails the test unless action appears at least once.
func (t *Trace) AssertCalled(tb testing.TB, name string) {
	tb.Helper()
	if t.Count(name) == 0 {
		tb.Fatalf("action %q was never called; trace: %v", name, t.Names())
	}
}

// AssertNotCalled fails the test if action appears at least once.
func (t *Trace) AssertNotCalled(tb testing.TB, name string) {
	tb.Helper()
	if n := t.Count(name); n > 0 {
		tb.Fatalf("action %q was called %d time(s); trace: %v", name, n, t.Names())
	}
}

// AssertOrder fails the test unless first appears before second.
func (t *Trace) AssertOrder(tb testing.TB, first, second string) {
	tb.Helper()
	calls := t.Calls()
	for _, c := range calls {
		if c.Action == first {
			for _, c2 := range calls {
				if c2.Action == second && c2.Seq > c.Seq {
					return
				}
			}
			tb.Fatalf("action %q never appears before %q", first, second)
			return
		}
	}
	tb.Fatalf("action %q was never called; trace: %v", first, t.Names())
}

// AssertAllSucceeded fails the test if any recorded call returned an error.
func (t *Trace) AssertAllSucceeded(tb testing.TB) {
	tb.Helper()
	for _, c := range t.Calls() {
		if c.Err != nil {
			tb.Fatalf("action %q failed: %v", c.Action, c.Err)
		}
	}
}

// AssertErrorsAt fails the test unless the named actions failed. Useful
// for asserting "the *only* failure was X".
func (t *Trace) AssertErrorsAt(tb testing.TB, want ...string) {
	tb.Helper()
	var got []string
	for _, c := range t.Calls() {
		if c.Err != nil {
			got = append(got, c.Action)
		}
	}
	if !slices.Equal(got, want) {
		tb.Fatalf("error trace mismatch\n  want: %v\n  got:  %v", want, got)
	}
}

// Dump renders the trace as a printable table. Call it inside
// t.Logf on failure, or use tb.Cleanup(func(){ if tb.Failed() { t.Dump(tb) } }).
func (t *Trace) Dump(tb testing.TB) {
	tb.Helper()
	var sb strings.Builder
	sb.WriteString("trace:\n")
	for _, c := range t.Calls() {
		status := "OK"
		if c.Err != nil {
			status = "ERR"
		}
		fmt.Fprintf(&sb, "  [%3d] %-30s %-4s %s\n", c.Seq, c.Action, status, c.Duration)
	}
	tb.Log(sb.String())
}
