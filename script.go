package testkit

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/nexssp/kernel/action"
)

var _ action.HookDispatcher[int, string] = (*Recorder[int, string])(nil)

// Recorder implements action.HookDispatcher with full goroutine concurrency safety.
type Recorder[Req, Res any] struct {
	mu           sync.RWMutex
	Retries      []RetryEvent[Req]
	CacheHits    int
	CacheMisses  int
	Coalesced    int
	Deduplicated int
}

type RetryEvent[Req any] struct {
	Request Req
	Attempt int
	Err     error
}

func (r *Recorder[Req, Res]) OnCacheHit(context.Context, Req, Res) {
	r.mu.Lock()
	r.CacheHits++
	r.mu.Unlock()
}

func (r *Recorder[Req, Res]) OnCacheMiss(context.Context, Req) {
	r.mu.Lock()
	r.CacheMisses++
	r.mu.Unlock()
}

func (r *Recorder[Req, Res]) OnRetry(_ context.Context, req Req, attempt int, err error) {
	r.mu.Lock()
	r.Retries = append(r.Retries, RetryEvent[Req]{req, attempt, err})
	r.mu.Unlock()
}

func (r *Recorder[Req, Res]) OnCoalesced(context.Context, Req) {
	r.mu.Lock()
	r.Coalesced++
	r.mu.Unlock()
}

func (r *Recorder[Req, Res]) OnDeduplicated(context.Context, Req) {
	r.mu.Lock()
	r.Deduplicated++
	r.mu.Unlock()
}

// Script returns a thread-safe deterministic action function.
func Script[Req, Res any](results ...Result[Res]) action.Fn[Req, Res] {
	if len(results) == 0 {
		panic("testkit.Script requires at least one result")
	}
	var mu sync.Mutex
	index := 0
	return func(context.Context, Req) (Res, error) {
		mu.Lock()
		result := results[index]
		if index < len(results)-1 {
			index++
		}
		mu.Unlock()
		return result.Value, result.Err
	}
}

type Result[Res any] struct {
	Value Res
	Err   error
}

func Success[Res any](value Res) Result[Res] { return Result[Res]{Value: value} }
func Failure[Res any](err error) Result[Res] { return Result[Res]{Err: err} }

func MustExecute[Req, Res any](t testing.TB, fn action.Fn[Req, Res], req Req) Res {
	t.Helper()
	res, err := fn(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected action error: %v", err)
	}
	return res
}

func MustError[Req, Res any](t testing.TB, fn action.Fn[Req, Res], req Req, want error) {
	t.Helper()
	_, err := fn(context.Background(), req)
	if !errors.Is(err, want) {
		t.Fatalf("expected error %v, got %v", want, err)
	}
}
