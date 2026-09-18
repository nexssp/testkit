// Package trace demonstrates the Trace hook.
//
// The point: recording what actually happened is the difference between
// "the pipeline produced the right final value" (weak) and "the
// pipeline invoked each step exactly once, in the correct order, with
// the right input at each step" (strong).
//
// Trace is a domainless testkit primitive. It works against any set of
// actions, however they were composed.
package trace

import (
	"context"
	"testing"

	"github.com/nexssp/kernel/action"
	"github.com/nexssp/testkit"
)

func TestTrace_ProvesCallOrder(t *testing.T) {
	trace := testkit.NewTrace()

	fetch := action.New("fetch", func(_ context.Context, in string) (string, error) {
		return "raw:" + in, nil
	}).AnyHook(trace.Hook()).Build()

	transform := action.New("transform", func(_ context.Context, in string) (string, error) {
		return "clean:" + in, nil
	}).AnyHook(trace.Hook()).Build()

	store := action.New("store", func(_ context.Context, in string) (string, error) {
		return "stored:" + in, nil
	}).AnyHook(trace.Hook()).Build()

	// Simulate a hand-composed pipeline (or a compiled flow — same trace).
	ctx := context.Background()
	r1, _ := fetch.Do(ctx, "A")
	r2, _ := transform.Do(ctx, r1)
	_, _ = store.Do(ctx, r2)

	trace.AssertSequence(t, "fetch", "transform", "store")
	trace.AssertOrder(t, "fetch", "store")
	trace.AssertAllSucceeded(t)
}
