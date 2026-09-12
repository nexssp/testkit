package testkit

import (
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestEventuallyReturnsWhenConditionIsImmediatelyTrue(t *testing.T) {
	var calls atomic.Int32

	Eventually(t, time.Second, time.Millisecond, func() bool {
		calls.Add(1)
		return true
	})

	if got := calls.Load(); got != 1 {
		t.Fatalf("condition calls = %d, want 1", got)
	}
}

func TestEventuallyRetriesUntilConditionBecomesTrue(t *testing.T) {
	var calls atomic.Int32

	Eventually(t, time.Second, time.Millisecond, func() bool {
		return calls.Add(1) >= 4
	})

	if got := calls.Load(); got != 4 {
		t.Fatalf("condition calls = %d, want 4", got)
	}
}

func TestEventuallyEvaluatesConditionSynchronously(t *testing.T) {
	var calls atomic.Int32

	Eventually(t, time.Second, time.Millisecond, func() bool {
		return calls.Add(1) == 2
	})

	if got := calls.Load(); got != 2 {
		t.Fatalf("condition calls = %d, want 2", got)
	}
}

func TestEventuallyNormalizesNonPositiveInterval(t *testing.T) {
	var calls atomic.Int32
	start := time.Now()

	Eventually(t, time.Second, 0, func() bool {
		return calls.Add(1) >= 2
	})

	if got := calls.Load(); got != 2 {
		t.Fatalf("condition calls = %d, want 2", got)
	}
	if elapsed := time.Since(start); elapsed < time.Millisecond {
		t.Fatalf("condition completed too quickly: interval normalization may have been bypassed")
	}
}

func TestEventuallyAllowsOneImmediateEvaluationWithNonPositiveTimeout(t *testing.T) {
	if os.Getenv("TESTKIT_EVENTUALLY_FAIL_ZERO_TIMEOUT") == "1" {
		Eventually(t, 0, time.Millisecond, func() bool { return false })
		return
	}

	cmd := testCommand(t, "TestEventuallyAllowsOneImmediateEvaluationWithNonPositiveTimeout")
	cmd.Env = append(os.Environ(), "TESTKIT_EVENTUALLY_FAIL_ZERO_TIMEOUT=1")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected subprocess to fail")
	}
	if !strings.Contains(string(output), "timeout=0s") {
		t.Fatalf("failure output = %q, want timeout detail", output)
	}
}

func TestEventuallyFailsAfterTimeout(t *testing.T) {
	if os.Getenv("TESTKIT_EVENTUALLY_FAIL_TIMEOUT") == "1" {
		Eventually(t, 20*time.Millisecond, time.Millisecond, func() bool { return false })
		return
	}

	cmd := testCommand(t, "TestEventuallyFailsAfterTimeout")
	cmd.Env = append(os.Environ(), "TESTKIT_EVENTUALLY_FAIL_TIMEOUT=1")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected subprocess to fail")
	}
	if !strings.Contains(string(output), "condition was false after 20ms") {
		t.Fatalf("failure output = %q, want timeout detail", output)
	}
}

func TestEventuallyRejectsNilCondition(t *testing.T) {
	if os.Getenv("TESTKIT_EVENTUALLY_FAIL_NIL") == "1" {
		Eventually(t, time.Second, time.Millisecond, nil)
		return
	}

	cmd := testCommand(t, "TestEventuallyRejectsNilCondition")
	cmd.Env = append(os.Environ(), "TESTKIT_EVENTUALLY_FAIL_NIL=1")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected subprocess to fail")
	}
	if !strings.Contains(string(output), "nil condition") {
		t.Fatalf("failure output = %q, want nil-condition detail", output)
	}
}

func testCommand(t *testing.T, testName string) *exec.Cmd {
	t.Helper()
	return exec.Command(os.Args[0], "-test.run=^"+testName+"$", "-test.v")
}
