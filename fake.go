package testkit

import (
	"context"
	"sync"

	"github.com/nexssp/kernel/action"
)

// Fake returns an action that always succeeds with res. It is the
// domainless replacement for a hand-written test fixture: every example
// and every test that needs "an action that just returns X" uses this
// one function.
func Fake(name string, res any) action.AnyAction {
	return action.New[any, any](name, func(context.Context, any) (any, error) {
		return res, nil
	}).Build()
}

// FakeErr returns an action that always fails with err. Use it for
// "the happy path plus a fallback" tests, contract tests, and any
// pipeline that must be proven to abort cleanly.
func FakeErr(name string, err error) action.AnyAction {
	return action.New[any, any](name, func(context.Context, any) (any, error) {
		return nil, err
	}).Build()
}

// FakeSeq returns an action that yields each value in order, then
// repeats the last one. It is the "flaky on first call, then
// recovers" helper every retry test needs. Safe for concurrent use.
func FakeSeq(name string, values ...any) action.AnyAction {
	if len(values) == 0 {
		panic("testkit.FakeSeq: at least one value required")
	}
	var (
		mu sync.Mutex
		i  int
	)
	return action.New[any, any](name, func(context.Context, any) (any, error) {
		mu.Lock()
		v := values[i]
		if i < len(values)-1 {
			i++
		}
		mu.Unlock()
		return v, nil
	}).Build()
}
