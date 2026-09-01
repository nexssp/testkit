package testkit

import (
	"context"
	"testing"

	"github.com/nexssp/kernel/action"
)

// BenchAction benchmarks a built action's pure execution speed without transport overhead.
func BenchAction[Req, Res any](b *testing.B, act *action.BuiltAction[Req, Res], req Req) {
	b.Helper()
	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		ctx := context.Background()
		for pb.Next() {
			if _, err := act.Do(ctx, req); err != nil {
				b.Errorf("unexpected error during benchmark: %v", err)
			}
		}
	})
}

// BenchHTTP benchmarks the complete HTTP stack over in-memory sockets.
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
