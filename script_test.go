package testkit_test

import (
	"context"
	"errors"
	"testing"

	"github.com/nexssp/testkit"
)

var errTerminal = errors.New("terminal error")

func TestScript_SequenceAndMustHelpers(t *testing.T) {
	t.Parallel()

	fn := testkit.Script[int, string](
		testkit.Success("step-1"),
		testkit.Failure[string](errTerminal),
		testkit.Success("step-final"),
	)

	// Call 1 -> Step 1
	res1 := testkit.MustExecute(t, fn, 1)
	if res1 != "step-1" {
		t.Fatalf("expected step-1, got %q", res1)
	}

	// Call 2 -> Step 2 (Error)
	testkit.MustError(t, fn, 2, errTerminal)

	// Call 3 -> Step Final
	res3 := testkit.MustExecute(t, fn, 3)
	if res3 != "step-final" {
		t.Fatalf("expected step-final, got %q", res3)
	}

	// Call 4 -> Repeats last result
	res4 := testkit.MustExecute(t, fn, 4)
	if res4 != "step-final" {
		t.Fatalf("expected step-final to repeat, got %q", res4)
	}
}

func TestRecorder_AllHookEvents(t *testing.T) {
	t.Parallel()

	rec := new(testkit.Recorder[string, string])
	ctx := context.Background()

	rec.OnCacheHit(ctx, "req", "cached")
	rec.OnCacheMiss(ctx, "req")
	rec.OnCoalesced(ctx, "req")
	rec.OnDeduplicated(ctx, "req")
	rec.OnRetry(ctx, "req", 1, errors.New("err"))

	if rec.CacheHits != 1 || rec.CacheMisses != 1 || rec.Coalesced != 1 || rec.Deduplicated != 1 {
		t.Fatalf("recorder counter mismatch: %+v", rec)
	}
	if len(rec.Retries) != 1 || rec.Retries[0].Attempt != 1 {
		t.Fatalf("recorder retry tracking failed: %+v", rec.Retries)
	}
}
