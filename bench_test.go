package testkit_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/nexssp/kernel/action"
	"github.com/nexssp/testkit"
)

func BenchmarkHelpers_Smoke(b *testing.B) {
	b.Run("Action", func(b *testing.B) {
		act := action.New("fast.action", func(_ context.Context, n int) (int, error) {
			return n * 2, nil
		}).Build()
		testkit.BenchAction(b, act, 42)
	})

	b.Run("HTTP", func(b *testing.B) {
		mux := http.NewServeMux()
		mux.HandleFunc("GET /ping", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		suite := testkit.NewWithHandler(b, mux)
		testkit.BenchHTTP(b, suite, "GET", "/ping", nil)
	})
}
