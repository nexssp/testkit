// Package testkit provides deterministic, fluent test harnesses for both
// nexssp actions and standard Go http.Handler implementations.
//
// Layout of this package:
//
//   - suite.go           — Suite lifecycle: New, NewWithHandler, NewE2E,
//     global headers/cookies, context bridge.
//   - request.go         — Request builder and every With* method;
//     Suite's HTTP verb methods (GET, POST, …).
//   - response.go        — Response and every fluent assertion.
//   - traverse.go        — dot-path resolution used by HasField and
//     ExpectArrayLen.
//   - chaos_bridge.go    — WithChaos, the only bridge from this package
//     into testkit/chaos.
//   - async.go           — WaitForJSON.
//   - bench.go           — BenchHTTP.
//   - load.go            — Suite.LoadTest.
//   - load_bg.go         — Suite.StartBackgroundLoad.
//   - sse.go, sse_wait.go — Server-Sent Events capture and waits.
//   - smoke.go           — RunSmokeTests.
//
// Generic helpers (Eventually, GoldenJSON, Trace, Fake, Script, Recorder,
// Simulate, AssertContracts, BenchAction, xctx test contexts, xerr
// assertions) live in github.com/nexssp/kernel/xtest.
//
// Subpackages (imported on demand only):
//
//   - chaos — stochastic latency/error/panic injection.
//   - rpc   — JSON-RPC 2.0 test client over net.Pipe.
//   - examples — runnable, self-contained demonstrations of the API.
package testkit
