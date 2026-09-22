# nexssp/testkit

[![Go Reference](https://pkg.go.dev/badge/github.com/nexssp/testkit.svg)](https://pkg.go.dev/github.com/nexssp/testkit)
[![CI](https://github.com/nexssp/testkit/actions/workflows/ci.yml/badge.svg)](https://github.com/nexssp/testkit/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)

**Test the real boundary, not a mock of it.**

`nexssp/testkit` is a Go testing toolkit for real HTTP integration tests, black-box E2E tests, SSE streams, load, chaos, and JSON-RPC/stdio transports.

It uses the standard `testing` package and standard `net/http` interfaces. It does not require a third-party mock framework.

## Two packages, two scopes

Testing a Nexssp system means testing two different layers, and there are two different packages for that:

| Layer | Package | What it owns |
| --- | --- | --- |
| HTTP, SSE, JSON-RPC, load, chaos | `github.com/nexssp/testkit` (this package) | Wire-protocol test harnesses against a live `http.Handler`, a running process, or an in-memory `net.Pipe` |
| Action primitives | `github.com/nexssp/kernel/xtest`, `.../xtest/ktest` | Polling (`Eventually`), golden files, call tracing, action fakes, scripted responses, hook recording, concurrency barriers, benchmark helpers, context and error assertions |

Rule of thumb:

- If the test touches HTTP, SSE, JSON-RPC, a network socket, or a load generator — it belongs in `testkit`.
- If the test touches `action.BuiltAction`, `iter.Seq2`, hooks, `xctx`, or `xerr` — it belongs in `kernel/xtest` or `kernel/xtest/ktest`.

There is no overlap. `testkit` does not re-export kernel helpers, and `kernel/xtest` does not touch `net/http`.

## What testkit provides

| Target | Constructor or package | Scope |
| --- | --- | --- |
| `nexssp/kernel` actions | `testkit.New` | In-process HTTP integration with mounted actions |
| Any Go router or handler | `testkit.NewWithHandler` | Router and middleware integration |
| Running local, LAN, container, staging, or production URL | `testkit.NewE2E` | Black-box HTTP E2E |
| JSON-RPC or stdio server | `testkit/rpc` | Line-delimited JSON-RPC over an in-memory pipe |
| Action fault injection | `testkit/chaos`, `testkit.WithChaos` | Stochastic latency, errors, and panics |
| HTTP load | `testkit.LoadTest`, `testkit.StartBackgroundLoad` | In-process stress and profiling |

## What kernel/xtest provides

Use these directly. They are not re-exported here.

| Need | API |
| --- | --- |
| Wait for arbitrary state | `xtest.Eventually(t, timeout, cond)` |
| Custom poll interval | `xtest.EventuallyEvery(t, timeout, interval, cond)` |
| Error-returning variants | `xtest.EventuallyE`, `xtest.EventuallyEveryE` |
| Wait for a value | `xtest.WaitForValue[T](t, timeout, want, get)` |
| Golden-file snapshot | `xtest.GoldenJSON(t, name, got)` with `-xtest.update` |
| Assert callback order | `ktest.Trace`, `Trace.RequireSequence`, `RequireOrder` |
| Concurrency barrier | `ktest.Simulate(t, act, req, n, assertFn)` |
| Structural action contracts | `ktest.AssertContracts(t, actions)` |
| Scripted action responses | `ktest.Script`, `ktest.Success`, `ktest.Failure` |
| Record retry / cache / coalesce | `ktest.Recorder[Req, Res]` |
| Action stubs | `ktest.Fake`, `ktest.Echo`, `ktest.Returns`, `ktest.Fails`, `ktest.Sequence`, `ktest.Flaky` |
| Single-action run | `ktest.Run(tb, act, ctx, req)` |
| Action benchmark | `ktest.BenchAction(b, act, req)` |
| xctx-populated test contexts | `ktest.Ctx`, `ktest.CtxWithAuth`, ... |
| xerr assertions | `ktest.RequireNoError`, `ktest.RequireErrorKind`, ... |
| Parallel runner | `xtest.RunParallel(tb, n, fn)` |
| Goroutine leak check | `xtest.RequireNoGoroutineLeak(tb, base, timeout)` |

## Installation

```bash
go get github.com/nexssp/testkit@latest
go get github.com/nexssp/kernel@latest
```

## Quick decision guide

### Choose a constructor

```go
// Kernel actions, in memory.
suite := testkit.New(t, GetUser, CreateOrder)

// Any standard net/http.Handler.
suite := testkit.NewWithHandler(t, mux)

// A running process, container, LAN machine, or staging deployment.
suite := testkit.NewE2E(t, "http://127.0.0.1:8080")
```

### Choose an assertion

| Requirement | API | Package |
| --- | --- | --- |
| One synchronous HTTP request | `suite.GET(...).Do().ExpectOK()` | testkit |
| Decode one response | `response.Into(&dst)` | testkit |
| Inspect nested JSON | `response.HasField("data.id", value)` | testkit |
| Poll an HTTP JSON endpoint | `testkit.WaitForJSON(...)` | testkit |
| Wait for a named SSE event | `stream.WaitFor(...)` | testkit |
| Wait for an SSE substring | `stream.WaitForData(...)` | testkit |
| Match structured SSE criteria | `stream.WaitForSSE(...)` | testkit |
| Probe all action routes over HTTP | `testkit.RunSmokeTests(...)` | testkit |
| Test JSON-RPC/stdio | `testkit/rpc` | testkit |
| Wait for a non-HTTP condition | `xtest.Eventually(...)` | kernel/xtest |
| Snapshot a value as golden JSON | `xtest.GoldenJSON(...)` | kernel/xtest |
| Assert callback order | `ktest.Trace` | kernel/xtest/ktest |
| Force N goroutines through a barrier | `ktest.Simulate(...)` | kernel/xtest/ktest |
| Check structural action invariants | `ktest.AssertContracts(...)` | kernel/xtest/ktest |

## Cheatsheet

```go
// Immediate HTTP assertion.
suite.GET("/healthz").Do().ExpectOK()

// Request options.
suite.POST("/v1/orders").
    WithJSON(map[string]any{"sku": "A1"}).
    WithBearerToken("token").
    WithTenant("tenant-1").
    Do().
    ExpectCreated()

// Typed response.
var status StatusResponse
suite.GET("/api/status").Do().ExpectOK().Into(&status)

// Eventually consistent HTTP state.
status = testkit.WaitForJSON[StatusResponse](
    t,
    20*time.Second,
    250*time.Millisecond,
    func() (*testkit.Response, error) {
        return suite.GET("/api/status").DoE()
    },
    func(value StatusResponse) bool {
        return value.State == "completed"
    },
)

// SSE event matching.
stream := suite.ListenSSE("/api/events")
t.Cleanup(stream.Close)
event := stream.WaitForSSE(t, 20*time.Second, func(event testkit.SSEEvent) bool {
    return event.Event == "docker" && strings.Contains(event.Data, "completed")
})
_ = event

// Non-HTTP eventual condition (kernel xtest).
xtest.Eventually(t, 10*time.Second, func() bool {
    return service.IsReady()
})
```

## 1. Testing kernel actions

`testkit.New` mounts actions on an in-memory `httptest.Server`, applies the test context bridge, and registers cleanup with `t.Cleanup`.

```go
package orders_test

import (
    "context"
    "testing"

    "github.com/nexssp/kernel/action"
    "github.com/nexssp/testkit"
    "github.com/nexssp/transport/thttp"
)

type UserDTO struct {
    ID    string `json:"id" path:"id"`
    Email string `json:"email"`
}

func TestGetUser(t *testing.T) {
    getUser := action.New("user.get", func(_ context.Context, req UserDTO) (UserDTO, error) {
        return UserDTO{ID: req.ID, Email: "admin@nexss.com"}, nil
    }).Route(thttp.GET("/v1/users/{id}")).Build()

    suite := testkit.New(t, getUser)
    suite.GET("/v1/users/usr-42").
        Do().
        ExpectOK().
        HasField("id", "usr-42").
        HasField("email", "admin@nexss.com")
}
```

## 2. Testing any `http.Handler`

No kernel dependency is required. `NewWithHandler` works with the standard library and routers such as Chi, Gin, and Echo.

```go
func TestHealth(t *testing.T) {
    mux := http.NewServeMux()
    mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        _, _ = w.Write([]byte(`{"status":"healthy"}`))
    })

    suite := testkit.NewWithHandler(t, mux)
    suite.GET("/healthz").Do().ExpectOK().HasField("status", "healthy")
}
```

## 3. Black-box E2E testing

`NewE2E` uses the same request and assertion API against a running service.

```go
func TestStagingStatus(t *testing.T) {
    suite := testkit.NewE2E(t, "https://staging-api.example.com")

    suite.GET("/v1/system/status").
        WithHeader("X-Api-Key", os.Getenv("STAGING_API_KEY")).
        Do().
        ExpectOK().
        HasField("environment", "staging")
}
```

For a local LAN service:

```bash
JUMALU_URL=http://192.168.1.42:8080 go test -tags=integration -v ./integration/...
```

Use E2E tests for the real process, configuration, persistence, Docker, network, and provider boundaries. Keep them separate from fast in-process tests.

## 4. HTTP requests and assertions

```go
suite.POST("/v1/invoices").
    WithQuery("draft", "true").
    WithQueries(map[string]string{"currency": "USD"}).
    WithHeader("X-Trace-ID", "trace-123").
    WithBearerToken("jwt-token").
    WithTenant("tenant-corp").
    WithCookie("session_id", "sess-abc").
    WithJSON(map[string]any{
        "customer": "Acme Corp",
        "amount":   1250.50,
    }).
    Do().
    ExpectCreated()
```

For multipart uploads:

```go
suite.POST("/v1/documents/upload").
    WithMultipartFile("file", "invoice.pdf", []byte("%PDF-1.4...")).
    Do().
    ExpectCreated()
```

Available response operations:

```go
ExpectStatus(code)
ExpectSuccess()
ExpectOK()
ExpectCreated()
ExpectBadRequest()
ExpectUnauthorized()
ExpectForbidden()
ExpectNotFound()
ExpectHeader(name, value)
HasField("data.items.0.sku", "PROD-99")
ExpectArrayLen("data.items", 3)
ContainsString("Acme Corp")
Into(&typedValue)
ExpectErrorKind(kind, message)
```

`HasField` uses dot-separated paths and numeric array indexes. Use `Into` when a typed response is more readable or when the test needs several fields.

`DoE` returns an error instead of failing the test immediately and is useful in polling functions:

```go
response, err := suite.GET("/healthz").DoE()
```

## 5. Asynchronous HTTP and SSE assertions

### `WaitForJSON`

`WaitForJSON` polls an HTTP endpoint that exposes eventually consistent state. The request factory must build a fresh request for each attempt. Request and JSON-decoding errors are retried; if the timeout expires, the last observed error is included in the failure message.

```go
type StatusResponse struct {
    State string `json:"state"`
}

status := testkit.WaitForJSON[StatusResponse](
    t,
    20*time.Second,
    250*time.Millisecond,
    func() (*testkit.Response, error) {
        return suite.GET("/api/status").DoE()
    },
    func(value StatusResponse) bool {
        return value.State == "completed"
    },
)
```

For non-HTTP polling, use `xtest.Eventually` or `xtest.EventuallyEvery` from `github.com/nexssp/kernel/xtest`. Do not use polling to hide a deterministic failure.

### SSE

```go
stream := suite.ListenSSE("/events/telemetry")
t.Cleanup(stream.Close)

suite.POST("/v1/trigger-event", map[string]string{"msg": "ping"}).
    Do().
    ExpectOK()

// Named event.
event := stream.WaitFor(t, "message", 2*time.Second)

// Data substring.
event = stream.WaitForData(t, "ping", 2*time.Second)

// Structured predicate.
event = stream.WaitForSSE(t, 2*time.Second, func(event testkit.SSEEvent) bool {
    return event.Event == "message" && strings.Contains(event.Data, "ping")
})
```

For MCP-style SSE handshakes:

```go
endpoint := stream.Endpoint(t, 2*time.Second)
```

For POST plus SSE protocols such as MCP Streamable HTTP or A2A, use `ListenSSEWithRequest`.

## 6. Route smoke testing

`RunSmokeTests` discovers mounted action routes, synthesizes minimal payloads, substitutes path parameters, and fails on unhandled `5xx` responses.

```go
func TestAllRoutes(t *testing.T) {
    testkit.RunSmokeTests(t, []action.AnyAction{
        CreateOrder,
        GetOrder,
        ListOrders,
    })
}
```

A `2xx`, `4xx`, or other intentional non-`5xx` response passes. This verifies that endpoints do not panic or fail through an unhandled server error; it does not replace behavioral endpoint tests.

## 7. Load and latency testing

`LoadTest` runs in-process traffic and reports request count, RPS, error rate, and latency percentiles.

```go
result := suite.LoadTest(t, testkit.LoadConfig{
    Concurrency: 16,
    Duration:    3 * time.Second,
    Method:      http.MethodGet,
    Path:        "/v1/orders/ord-1",
})

t.Logf("requests=%d rps=%.2f errors=%.2f%% p50=%v p95=%v p99=%v",
    result.TotalRequests,
    result.RPS,
    result.ErrorRate*100,
    result.P50,
    result.P95,
    result.P99,
)
```

`StartBackgroundLoad` is useful while collecting profiles or inspecting metrics:

```go
stop := suite.StartBackgroundLoad(testkit.LoadConfig{
    Concurrency: 8,
    Method:      http.MethodGet,
    Path:        "/v1/orders/ord-1",
})
defer stop()
```

Treat load-test thresholds as environment-specific measurements, not universal guarantees.

## 8. Chaos and fault injection

`testkit/chaos` injects controlled delay, errors, and panics into action execution.

```go
actions := testkit.WithChaos([]action.AnyAction{CreateOrder}, chaos.Config{
    Enabled:   true,
    ErrorRate: 0.25,
    MaxDelay:  150 * time.Millisecond,
    PanicRate: 0.02,
})

suite := testkit.New(t, actions)
suite.POST("/v1/orders", map[string]any{"sku": "A"}).
    Do().
    ExpectSuccess()
```

Use chaos tests to verify retry, timeout, circuit-breaker, panic-recovery, and error-boundary behavior. Keep randomness bounded and use assertions that tolerate the configured fault rate.

## 9. JSON-RPC and stdio transports

Use `testkit/rpc` for line-delimited JSON-RPC transports such as MCP stdio. It uses an in-memory `net.Pipe` and a real JSON-RPC client.

```go
func TestMCPStdio(t *testing.T) {
    client := rpc.DialJSONRPC(t, func(ctx context.Context, in io.Reader, out io.Writer) error {
        return mcpServer.Serve(ctx, in, out)
    })

    response := client.Call("tools/list", nil, 1)

    var result struct {
        Tools []struct {
            Name string `json:"name"`
        } `json:"tools"`
    }
    response.BindResult(t, &result)

    if len(result.Tools) == 0 {
        t.Fatal("expected at least one tool")
    }
}
```

The RPC client supports:

```go
client.Call(method, params, id)
client.Notify(method, params)
client.CallRaw(rawJSON)
response.BindResult(t, &destination)
response.Error
```

## Testing guidance

Keep tests in layers:

1. **Unit tests** for pure functions and deterministic action behavior.
1. **In-process integration tests** with `New` or `NewWithHandler`.
1. **Black-box E2E tests** with `NewE2E` against a real process or deployment.
1. **Load and chaos tests** as explicit, separately named test groups.

Use build tags or environment gates for tests requiring Docker, external providers, a LAN service, or a running binary:

```bash
go test ./...
go test -race ./...
go test -tags=integration -v ./integration/...
```

Avoid putting credentials in source code or logs. Use environment variables and explicit test configuration. A trusted-LAN E2E test is not an authorization test and must not be presented as one.

## Project status and performance claims

`testkit` provides testing primitives and measurement tools. It does not guarantee that an application is production-ready, zero-allocation on every path, or capable of a particular scale. Those properties depend on the application, dependencies, deployment, workload, and measured results.

Use benchmarks, race tests, load tests, and production-like E2E tests to establish claims for a specific system.

## License

Apache License 2.0. See [LICENSE](LICENSE) for details.
