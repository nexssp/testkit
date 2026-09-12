# nexssp/testkit

[![Go Reference](https://pkg.go.dev/badge/github.com/nexssp/testkit.svg)](https://pkg.go.dev/github.com/nexssp/testkit)
[![CI](https://github.com/nexssp/testkit/actions/workflows/ci.yml/badge.svg)](https://github.com/nexssp/testkit/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)

**Test the real boundary, not a mock of it.**

`nexssp/testkit` is a Go testing toolkit for real HTTP integration tests, black-box E2E tests, SSE streams, action contracts, concurrency, load, chaos, and JSON-RPC/stdio transports.

It uses the standard `testing` package and standard `net/http` interfaces. It does not require a third-party mock framework.

## What it tests

| Target | Constructor or package | Scope |
| --- | --- | --- |
| `nexssp/kernel` actions | `testkit.New` | In-process HTTP integration with mounted actions |
| Any Go router or handler | `testkit.NewWithHandler` | Router and middleware integration |
| Running local, LAN, container, staging, or production URL | `testkit.NewE2E` | Black-box HTTP E2E |
| JSON-RPC or stdio server | `testkit/rpc` | Line-delimited JSON-RPC over an in-memory pipe |
| Action resilience and concurrency | `Simulate`, `Script`, `Recorder`, `WithChaos` | Deterministic and stress-oriented tests |

## Installation

```bash
go get github.com/nexssp/testkit@latest
```

## Quick decision guide

### Choose a constructor

```go
// Kernel actions, in memory.
suite := testkit.New(t, GetUser, CreateOrder )

// Any standard net/http.Handler.
suite := testkit.NewWithHandler(t, mux )

// A running process, container, LAN machine, or staging deployment.
suite := testkit.NewE2E(t, "http://127.0.0.1:8080" )
```

### Choose an assertion

| Requirement | API |
| --- | --- |
| One synchronous HTTP request | `suite.GET(...).Do().ExpectOK()` |
| Decode one response | `response.Into(&dst)` |
| Inspect nested JSON | `response.HasField("data.id", value)` |
| Wait for arbitrary state | `testkit.Eventually(...)` |
| Poll an HTTP JSON endpoint | `testkit.WaitForJSON(...)` |
| Wait for a named SSE event | `stream.WaitFor(...)` |
| Wait for an SSE substring | `stream.WaitForData(...)` |
| Match structured SSE criteria | `stream.WaitForSSE(...)` |
| Probe all action routes | `testkit.RunSmokeTests(...)` |
| Test JSON-RPC/stdio | `testkit/rpc` |

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

// Generic eventual condition.
testkit.Eventually(t, 10*time.Second, 100*time.Millisecond, func() bool {
    return service.IsReady()
})

// SSE event matching.
stream := suite.ListenSSE("/api/events")
t.Cleanup(stream.Close)
event := stream.WaitForSSE(t, 20*time.Second, func(event testkit.SSEEvent) bool {
    return event.Event == "docker" && strings.Contains(event.Data, "completed")
})
_ = event
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
    }).Route(thttp.GET("/v1/users/{id}" )).Build()

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
func TestHealth(t *testing.T ) {
    mux := http.NewServeMux( )
    mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request ) {
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
    suite := testkit.NewE2E(t, "https://staging-api.example.com" )

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
suite.POST("/v1/invoices" ).
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

Available response operations include:

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

### `Eventually`

Use `Eventually` for a condition that is not specifically HTTP or SSE. It evaluates immediately, then waits between attempts. It starts no goroutines.

```go
testkit.Eventually(t, 10*time.Second, 100*time.Millisecond, func() bool {
    return engine.State() == "completed"
})
```

Use polling only for genuinely asynchronous behavior. Do not use it to hide a deterministic failure.

### `WaitForJSON`

Use `WaitForJSON` when an HTTP endpoint exposes eventually consistent state. The request factory must build a fresh request for each attempt.

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

Request and JSON-decoding errors are retried. If the timeout expires, the last observed error is included in the failure when one exists.

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

## 7. Architectural contracts

`AssertContracts` checks system-wide action invariants such as duplicate names, missing bindings, and malformed API payload types.

```go
testkit.AssertContracts(t, allActions)
```

## 8. Deterministic scripting and hook recording

`Script` supplies deterministic sequential results for retry, circuit-breaker, and fallback tests.

```go
scripted := testkit.Script[int, string](
    testkit.Failure[string](xerr.Unavailable("temporary failure")),
    testkit.Failure[string](xerr.Unavailable("temporary failure")),
    testkit.Success("recovered"),
)

act := action.New("order.settle", scripted).
    Retry(3, action.ConstantBackoff(time.Millisecond)).
    Build()
```

`Recorder` captures retry, cache, coalescing, and deduplication events.

```go
rec := new(testkit.Recorder[int, string])
```

Attach it through the action hook interfaces and assert on the recorded events after execution.

## 9. Concurrency and thundering-herd tests

`Simulate` starts concurrent workers behind a synchronized barrier.

```go
var databaseCalls atomic.Int32

fetchPrice := action.New("price.fetch", func(_ context.Context, sku string) (float64, error) {
    databaseCalls.Add(1)
    time.Sleep(20 * time.Millisecond)
    return 199.99, nil
}).Dedup(func(sku string) string { return sku }).Build()

testkit.Simulate(t, fetchPrice, "SKU-1", 50, func(t testing.TB, price float64, err error) {
    if err != nil || price != 199.99 {
        t.Errorf("unexpected result: %v, err=%v", price, err)
    }
})

if got := databaseCalls.Load(); got != 1 {
    t.Fatalf("expected one underlying call, got %d", got)
}
```

## 10. Load and latency testing

`LoadTest` runs in-process traffic and reports request count, RPS, error rate, and latency percentiles.

```go
result := suite.LoadTest(t, testkit.LoadConfig{
    Concurrency: 16,
    Duration:    3 * time.Second,
    Method:      http.MethodGet,
    Path:        "/v1/orders/ord-1",
} )

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
} )
defer stop()
```

Treat load-test thresholds as environment-specific measurements, not universal guarantees.

## 11. Chaos and fault injection

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

## 12. JSON-RPC and stdio transports

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
