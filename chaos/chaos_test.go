package chaos_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nexssp/kernel/action"
	"github.com/nexssp/testkit/chaos"
)

func TestChaos_TransientErrorInjection(t *testing.T) {
	t.Parallel()

	act := action.New("chaos.target", func(_ context.Context, req string) (string, error) {
		return "ok:" + req, nil
	}).
		Use(chaos.Inject[string, string](chaos.Config{
			Enabled:   true,
			ErrorRate: 1.0,
		})).
		Build()

	_, err := act.Do(context.Background(), "input")
	if err == nil {
		t.Fatal("expected chaos error injection, got nil")
	}
}

func TestChaos_LatencyDelay(t *testing.T) {
	t.Parallel()

	act := action.New("chaos.latency", func(_ context.Context, _ struct{}) (string, error) {
		return "ok", nil
	}).
		Use(chaos.Inject[struct{}, string](chaos.Config{
			Enabled:  true,
			MaxDelay: 20 * time.Millisecond,
		})).
		Build()

	start := time.Now()
	res, err := act.Do(context.Background(), struct{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != "ok" {
		t.Fatalf("expected 'ok', got %q", res)
	}
	if time.Since(start) > 500*time.Millisecond {
		t.Fatal("chaos delay exceeded expected bounds")
	}
}

func TestChaos_PanicRecovery(t *testing.T) {
	t.Parallel()

	var panicRan atomic.Bool
	act := action.New("chaos.panic", func(_ context.Context, _ struct{}) (string, error) {
		return "ok", nil
	}).
		Use(chaos.Inject[struct{}, string](chaos.Config{
			Enabled:   true,
			PanicRate: 1.0,
		})).
		AnyHook(action.AnyHook{
			OnPanic: func(_ context.Context, _, _ any, _ *action.Meta) {
				panicRan.Store(true)
			},
		}).
		Build()

	_, err := act.Do(context.Background(), struct{}{})
	if err == nil {
		t.Fatal("expected error from recovered panic")
	}
	if !panicRan.Load() {
		t.Fatal("expected OnPanic hook to execute")
	}
}
