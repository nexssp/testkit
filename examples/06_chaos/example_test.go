package chaos

import (
	"context"
	"testing"
	"time"

	"github.com/nexssp/kernel/action"
	"github.com/nexssp/kernel/xerr"
	"github.com/nexssp/testkit/chaos"
)

func TestChaos_DrivesRetryPath(t *testing.T) {
	var attempts int

	act := action.New("flaky.upstream", func(_ context.Context, _ string) (string, error) {
		attempts++
		if attempts < 3 {
			return "", xerr.Unavailable("upstream down")
		}
		return "recovered", nil
	}).
		Use(chaos.Inject[string, string](chaos.Config{
			Enabled:   true,
			PanicRate: 0,
			ErrorRate: 0,
		})).
		Retry(5, action.ConstantBackoff(time.Millisecond)).
		Build()

	res, err := act.Do(context.Background(), "input")
	if err != nil {
		t.Fatalf("expected recovery after retries, got %v", err)
	}
	if res != "recovered" {
		t.Fatalf("expected 'recovered', got %q", res)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestChaos_PanicIsRecoveredIntoError(t *testing.T) {
	act := action.New("panicky", func(_ context.Context, _ string) (string, error) {
		return "unreachable", nil
	}).
		Use(chaos.Inject[string, string](chaos.Config{
			Enabled:   true,
			PanicRate: 1.0,
		})).
		Build()

	_, err := act.Do(context.Background(), "input")
	if err == nil {
		t.Fatal("expected error from injected panic")
	}

	// xerr.PanicRecovery wraps the recovered panic as KindInternal.
	if xerr.KindFrom(err) != xerr.KindInternal {
		t.Fatalf("expected KindInternal, got %v", xerr.KindFrom(err))
	}
}
