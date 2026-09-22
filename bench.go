package testkit

import (
	"context"
	"testing"
)

// BenchHTTP benchmarks the complete HTTP stack over in-memory sockets.
// For pure action execution without transport, use xtest.BenchAction.
func BenchHTTP(b *testing.B, s *Suite, method, path string, payload any) {
	b.Helper()
	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		ctx := context.Background()
		for pb.Next() {
			req := s.Request(method, path)
			if payload != nil {
				req.WithJSON(payload)
			}
			res, err := req.DoContext(ctx)
			if err != nil {
				b.Errorf("request failed: %v", err)
				continue
			}
			if res.Status() >= 500 {
				b.Errorf("HTTP %d: %s", res.Status(), res.BodyString())
			}
		}
	})
}
