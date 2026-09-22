package testkit_test

import (
	"net/http"
	"testing"

	"github.com/nexssp/testkit"
)

func BenchmarkBenchHTTP_Smoke(b *testing.B) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ping", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	suite := testkit.NewWithHandler(b, mux)
	testkit.BenchHTTP(b, suite, "GET", "/ping", nil)
}
