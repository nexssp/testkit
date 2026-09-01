package testkit_test

import (
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nexssp/testkit"
)

func TestLoad_InProcessStressTest(t *testing.T) {
	t.Parallel()

	var counter atomic.Int64

	mux := http.NewServeMux()
	mux.HandleFunc("GET /benchmark/fast", func(w http.ResponseWriter, _ *http.Request) {
		counter.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	suite := testkit.NewWithHandler(t, mux)

	res := suite.LoadTest(t, testkit.LoadConfig{
		Concurrency: 8,
		Duration:    150 * time.Millisecond,
		Method:      "GET",
		Path:        "/benchmark/fast",
	})

	if res.TotalRequests == 0 {
		t.Fatal("expected at least 1 request completed during load test")
	}
	if res.Errors != 0 {
		t.Fatalf("expected 0 errors, got %d", res.Errors)
	}
	if res.RPS <= 0 {
		t.Fatalf("expected positive RPS, got %.2f", res.RPS)
	}
	if res.P50 <= 0 || res.P95 <= 0 || res.P99 <= 0 {
		t.Fatalf("invalid latency percentiles: P50=%v P95=%v P99=%v", res.P50, res.P95, res.P99)
	}
}

func TestLoad_StartBackgroundLoad(t *testing.T) {
	t.Parallel()

	var hitCount atomic.Int64

	mux := http.NewServeMux()
	mux.HandleFunc("GET /ping", func(w http.ResponseWriter, _ *http.Request) {
		hitCount.Add(1)
		w.WriteHeader(http.StatusOK)
	})

	suite := testkit.NewWithHandler(t, mux)

	stop := suite.StartBackgroundLoad(testkit.LoadConfig{
		Concurrency: 4,
		Method:      "GET",
		Path:        "/ping",
	})

	time.Sleep(100 * time.Millisecond)
	stop()

	if hitCount.Load() == 0 {
		t.Fatal("expected background traffic generator to register hits")
	}
}
