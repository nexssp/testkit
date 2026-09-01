# nexssp/testkit

[![Go Reference](https://pkg.go.dev/badge/github.com/nexssp/testkit.svg)](https://pkg.go.dev/github.com/nexssp/testkit)
[![CI](https://github.com/nexssp/testkit/actions/workflows/ci.yml/badge.svg)](https://github.com/nexssp/testkit/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)

**Stop testing against mocks. Test against your real HTTP stack.**

`testkit` is the Go testing sidekick that replaces brittle mock layers with real, in-memory HTTP integration tests — **in milliseconds**.

One harness works with:

| Target | What you get |
|---|---|
| **`nexssp/kernel` actions** | Auto-mount actions, context bridge for tenant/user, route discovery |
| **Standard `http.Handler`** | Works with Chi, Gin, Echo, stdlib `http.ServeMux` — zero kernel required |
| **Live E2E URLs** | Black-box staging/prod tests using the exact same fluent DSL |
| **JSON-RPC / Stdio transports** | Test line-delimited protocols (MCP stdio, custom JSON-RPC) with a real client |

### Why testkit outperforms ordinary testing setups

- ⚡ **Faster than mock-heavy tests** — no reflection on hot paths, no generated mocks, no network hops
- 🔥 **Real HTTP server in memory** — exercise your real routing, middleware, and transport
- 🚫 **Zero third-party mock frameworks** — no `testify/mock`, no mock files, no brittle `MockRepository`
- 🔍 **Automatic route smoke tests** — discover and probe every endpoint in milliseconds
- 📊 **In-process load testing** — P50/P95/P99 without `k6` or `wrk`
- 📡 **SSE capture** — test realtime event streams natively
- 💣 **Chaos injection** — latency, 503s, and panics with a few lines
- 🧩 **Deterministic retry scripting** — unit-test circuit breakers without fakes
- 🔌 **JSON-RPC / Stdio testing** — test non-HTTP transports with the same real-client style

### The developer experience

```go
suite := testkit.New(t, GetOrder)

suite.GET("/v1/orders/ord-123").
    Do().
    ExpectOK().
    HasField("customer.tier", "VIP").
    HasField("items.0.sku", "PROD-99").
    ExpectArrayLen("items", 3)
```

Same DSL, three modes:

```go
// 1. nexssp/kernel actions
suite := testkit.New(t, CreateOrder, GetOrder)

// 2. Any standard http.Handler
suite := testkit.NewWithHandler(t, mux)

// 3. Live E2E
suite := testkit.NewE2E(t, "https://staging-api.example.com")
```

## Key Highlights

- **Dual-Mode Operation:** Works natively with `nexssp/kernel` actions or as a standalone integration toolkit for **any** standard `http.Handler` (Chi, Gin, Echo, stdlib `http.ServeMux`) or live E2E staging URLs.
- **Zero Third-Party Mock Frameworks:** No `testify/mock`, no generated mock files, no reflection on hot paths.
- **Deterministic Sequential Scripting:** Script sequential results (e.g. Failure $\to$ Failure $\to$ Success) for testing retry, backoff, and circuit breaker policies.
- **Automated Smoke Testing:** Discover and probe all registered routes in parallel to detect panics, invalid path regexes, and unhandled `5xx` internal errors in milliseconds.
- **Built-in Resilience & Stress Testing:** Run concurrency barriers (thundering-herd simulations), stochastic chaos injection (latency, 503s, panics), and benchmark execution speed with zero allocations.

---

## Installation

```bash
go get github.com/nexssp/testkit@latest
```

---

## Table of Contents

1. [Quick Start: Testing `nexssp` Actions](#1-quick-start-testing-nexssp-actions)
2. [Quick Start: Standalone with Any `http.Handler`](#2-quick-start-standalone-with-any-httphandler)
3. [Black-Box E2E Testing](#3-black-box-e2e-testing)
4. [Fluent HTTP Request & Assertion API](#4-fluent-http-request--assertion-api)
5. [Automated Route Smoke Testing](#5-automated-route-smoke-testing)
6. [Architectural Contract Verification](#6-architectural-contract-verification)
7. [Testing Server-Sent Events (SSE)](#7-testing-server-sent-events-sse)
8. [Deterministic Scripting & Hook Event Recording](#8-deterministic-scripting--hook-event-recording)
9. [Concurrency & Thundering-Herd Barriers](#9-concurrency--thundering-herd-barriers)
10. [In-Process Load Testing & P99 Latency Profiling](#10-in-process-load-testing--p99-latency-profiling)
11. [Chaos & Fault Injection](#11-chaos--fault-injection)
12. [Testing JSON-RPC / Stdio Transports](#12-testing-json-rpc--stdio-transports)

---

## 1. Quick Start: Testing `nexssp` Actions

When testing `nexssp/kernel` actions, `testkit.New(t, actions...)` automatically boots an in-memory loopback HTTP server (`httptest.Server`), mounts the actions, attaches a context bridge for tenant/user propagation, and manages teardown via `t.Cleanup`.

```go
package main_test

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

var GetUser = action.New("user.get", func(ctx context.Context, req UserDTO) (UserDTO, error) {
    return UserDTO{ID: req.ID, Email: "admin@nexss.com"}, nil
}).Route(thttp.GET("/v1/users/{id}")).Build()

func TestUserEndpoint(t *testing.T) {
    suite := testkit.New(t, GetUser)

    suite.GET("/v1/users/usr-42").
        Do().
        ExpectOK().
        HasField("id", "usr-42").
        HasField("email", "admin@nexss.com")
}
```

---

## 2. Quick Start: Standalone with Any `http.Handler`

You can use `testkit` without `nexssp/kernel`. Pass any router or handler implementing standard `net/http.Handler` to `testkit.NewWithHandler(t, handler)`.

### With Standard `http.ServeMux` (Go 1.22+)
```go
func TestStdlibMux(t *testing.T) {
    mux := http.NewServeMux()
    mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusOK)
        _, _ = w.Write([]byte(`{"status":"healthy","uptime_sec":3600}`))
    })

    suite := testkit.NewWithHandler(t, mux)

    suite.GET("/healthz").
        Do().
        ExpectOK().
        HasField("status", "healthy").
        HasField("uptime_sec", 3600)
}
```

### With Chi, Gin, or Echo
```go
func TestChiRouter(t *testing.T) {
    r := chi.NewRouter()
    r.Post("/api/v1/orders", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusCreated)
        _, _ = w.Write([]byte(`{"order_id":"ord-900","items":3}`))
    })

    suite := testkit.NewWithHandler(t, r)

    suite.POST("/api/v1/orders", map[string]any{"sku": "A1"}).
        Do().
        ExpectCreated().
        HasField("order_id", "ord-900").
        HasField("items", 3)
}
```

---

## 3. Black-Box E2E Testing

Use `testkit.NewE2E(t, baseURL)` to run the same assertions against a remote container, staging server, or live deployment.

```go
func TestStagingDeployment_E2E(t *testing.T) {
    suite := testkit.NewE2E(t, "https://staging-api.example.com")

    suite.GET("/v1/system/status").
        WithHeader("X-Api-Key", "secret-key").
        Do().
        ExpectOK().
        HasField("environment", "staging")
}
```

---

## 4. Fluent HTTP Request & Assertion API

### Request Builder Capabilities
```go
suite.POST("/v1/invoices").
    WithQuery("draft", "true").
    WithQueries(map[string]string{"lang": "pl", "currency": "PLN"}).
    WithHeader("X-Custom-Trace", "trace-123").
    WithBearerToken("jwt-token-here").
    WithTenant("tenant-corp-pl").               // Injects X-Tenant-ID
    WithCookie("session_id", "sess-abc").
    WithJSON(map[string]any{
        "customer": "Acme Corp",
        "amount":   1250.50,
    }).
    Do()
```

### Multipart File Uploads
```go
suite.POST("/v1/documents/upload").
    WithMultipartFile("file", "invoice.pdf", []byte("%PDF-1.4...")).
    Do().
    ExpectCreated()
```

### Response Assertions & JSON Path Traversal
`testkit` uses dot-separated JSON path traversal for inspecting nested objects and arrays without unmarshaling into ad-hoc structs.

```go
suite.GET("/v1/orders/ord-123").
    Do().
    ExpectOK().
    ExpectHeader("Content-Type", "application/json").
    HasField("data.order.id", "ord-123").
    HasField("data.order.customer.tier", "VIP").
    HasField("data.order.items.0.sku", "PROD-99"). // Indexed array lookup
    ExpectArrayLen("data.order.items", 3).
    ContainsString("Acme Corp")
```

### Typed Deserialization & Structured Error Validation
```go
var order OrderResponse

suite.GET("/v1/orders/ord-123").
    Do().
    ExpectOK().
    Into(&order) // Unmarshals JSON response body into target struct

// Verify categorized AppError output (xerr.Kind)
suite.GET("/v1/orders/missing-id").
    Do().
    ExpectNotFound().
    ExpectErrorKind(xerr.KindNotFound, "order not found")
```

---

## 5. Automated Route Smoke Testing

`RunSmokeTests` inspects all mounted actions, synthesizes valid minimal payloads using struct reflection, substitutes path parameters (e.g. `{id}` $\to$ `test-id`), and executes baseline HTTP calls.

```go
func TestAllEndpoints_Smoke(t *testing.T) {
    allActions := []action.AnyAction{
        CreateOrder,
        GetOrder,
        ListOrders,
        SettleInvoice,
        DeleteInvoice,
    }

    // Probes every route with synthetic inputs to ensure no endpoint crashes
    testkit.RunSmokeTests(t, allActions)
}
```

> **Passing Criterion:** Any status code `< 500` (including `200 OK`, `201 Created`, `400 Bad Request`, or `401 Unauthorized`) passes. Status `5xx` (panic, nil pointer dereference, unhandled runtime error) fails the test.

---

## 6. Architectural Contract Verification

`AssertContracts` enforces system-wide design invariants across your codebase:

- **No Duplicate Action Names:** Prevents conflicting route registrations.
- **No Orphaned Actions:** Catches actions created without route bindings or hooks.
- **Valid Payload Structs:** Ensures requests are structured as `struct` or `map` types (preventing naked primitives on API boundaries).

```go
func TestSystemInvariants_Contracts(t *testing.T) {
    allActions := []action.AnyAction{
        CreateOrder,
        GetOrder,
        SettleInvoice,
    }

    testkit.AssertContracts(t, allActions)
}
```

---

## 7. Testing Server-Sent Events (SSE)

`testkit` provides a dedicated SSE capture engine that connects to a stream, confirms the initial handshake, and buffers incoming stream chunks:

```go
func TestRealtimeTelemetry_SSE(t *testing.T) {
    suite := testkit.New(t, TelemetryStreamAction)

    // Connects and waits for the initial HTTP handshake
    stream := suite.ListenSSE("/events/telemetry")
    defer stream.Close()

    // Trigger an event on the server
    suite.POST("/v1/trigger-event", map[string]string{"msg": "ping"}).Do().ExpectOK()

    // Block until a matching event arrives or timeout expires
    event := stream.WaitFor(t, "message", 2*time.Second)

    if !strings.Contains(event.Data, "ping") {
        t.Fatalf("unexpected SSE payload: %s", event.Data)
    }
}
```

For MCP-style SSE handshakes, `Endpoint()` extracts the `event: endpoint` data, and `WaitForData()` blocks until an event contains a substring:

```go
stream := suite.ListenSSE("/mcp/sse")
endpoint := stream.Endpoint(t, 2*time.Second) // "/mcp/message?sessionId=..."

// POST to endpoint...

msg := stream.WaitForData(t, "result", 2*time.Second)
```

---

## 8. Deterministic Scripting & Hook Event Recording

For unit-testing retry policies, circuit breakers, and deduplication layers without third-party mocks:

### Deterministic Action Scripting
`testkit.Script` generates an execution function that returns supplied results in sequence and repeats the final result.

```go
func TestRetryPolicy_RecoversOnThirdAttempt(t *testing.T) {
    // 1st attempt: 503 Unavailable
    // 2nd attempt: 503 Unavailable
    // 3rd attempt: 200 Success
    scriptedFn := testkit.Script[int, string](
        testkit.Failure[string](xerr.Unavailable("database connection timeout")),
        testkit.Failure[string](xerr.Unavailable("database connection timeout")),
        testkit.Success("order_settled"),
    )

    act := action.New("order.settle", scriptedFn).
        Retry(3, action.ConstantBackoff(1*time.Millisecond)).
        Build()

    res, err := act.Do(context.Background(), 42)
    if err != nil || res != "order_settled" {
        t.Fatalf("expected recovery on attempt 3, got res=%q, err=%v", res, err)
    }
}
```

### Hook Event Recording
`testkit.Recorder` tracks internal middleware events (cache hits, misses, retries, deduplication, and coalescing).

```go
func TestRecorder_CapturesRetryTelemetry(t *testing.T) {
    rec := new(testkit.Recorder[int, string])

    scriptedFn := testkit.Script[int, string](
        testkit.Failure[string](xerr.Unavailable("transient failure")),
        testkit.Success("recovered"),
    )

    act := action.New("recorder.test", scriptedFn).
        Retry(2, action.ConstantBackoff(0)).
        Build()

    act.AddAnyHook(action.AnyHook{
        OnRetry: func(ctx context.Context, req any, attempt int, err error, meta *action.Meta) {
            rec.OnRetry(ctx, req.(int), attempt, err)
        },
    })

    res, err := act.Do(context.Background(), 100)
    if err != nil || res != "recovered" {
        t.Fatalf("unexpected execution result: %v", err)
    }

    if len(rec.Retries) != 1 || rec.Retries[0].Attempt != 1 {
        t.Fatalf("expected 1 recorded retry event, got %+v", rec.Retries)
    }
}
```

---

## 9. Concurrency & Thundering-Herd Barriers

`testkit.Simulate` synchronizes $N$ worker goroutines behind an atomic start barrier to verify thread safety and request deduplication.

```go
func TestSingleflight_ThunderingHerdShield(t *testing.T) {
    var dbCalls atomic.Int32

    fetchPrice := action.New("price.fetch", func(ctx context.Context, sku string) (float64, error) {
        dbCalls.Add(1)
        time.Sleep(20 * time.Millisecond) // Simulated slow DB lookup
        return 199.99, nil
    }).
        Dedup(func(sku string) string { return sku }).
        Build()

    // Dispatch 50 concurrent requests simultaneously at the exact same millisecond
    testkit.Simulate(t, fetchPrice, "SKU-PROD-1", 50, func(t testing.TB, price float64, err error) {
        if err != nil || price != 199.99 {
            t.Errorf("unexpected worker result: %v, err=%v", price, err)
        }
    })

    // Verify only 1 database query executed
    if got := dbCalls.Load(); got != 1 {
        t.Fatalf("thundering herd shield failed: expected 1 DB call, got %d", got)
    }
}
```

---

## 10. In-Process Load Testing & P99 Latency Profiling

### In-Process Stress Testing
Execute load tests directly within `go test` and calculate throughput and percentile latencies without spawning external tools like `k6` or `wrk`.

```go
func TestEndpoint_LoadPerformance(t *testing.T) {
    suite := testkit.New(t, GetOrder)

    results := suite.LoadTest(t, testkit.LoadConfig{
        Concurrency: 16,
        Duration:    3 * time.Second,
        Method:      "GET",
        Path:        "/v1/orders/ord-1",
    })

    t.Logf("Total Reqs: %d | RPS: %.2f | Error Rate: %.2f%%",
        results.TotalRequests, results.RPS, results.ErrorRate*100)
    t.Logf("Latencies: P50=%v | P95=%v | P99=%v",
        results.P50, results.P95, results.P99)

    if results.ErrorRate > 0.01 {
        t.Fatalf("error rate exceeded 1%%: %.4f", results.ErrorRate)
    }
    if results.P95 > 50*time.Millisecond {
        t.Fatalf("P95 latency exceeded 50ms: %v", results.P95)
    }
}
```

### Background Load Generator for `pprof` Profiling
```go
func TestProfileUnderSustainedLoad(t *testing.T) {
    suite := testkit.New(t, GetOrder)

    // Runs simulated background traffic in a goroutine until stop() is called
    stop := suite.StartBackgroundLoad(testkit.LoadConfig{
        Concurrency: 8,
        Method:      "GET",
        Path:        "/v1/orders/ord-1",
    })
    defer stop()

    // Perform specific diagnostic assertions while the system is under sustained load
    time.Sleep(2 * time.Second)
}
```

---

## 11. Chaos & Fault Injection

`testkit/chaos` injects controlled failure modes directly into actions or suites to verify that your circuit breakers, retries, and error boundaries work under stress.

```go
import "github.com/nexssp/testkit/chaos"

func TestResilience_UnderNetworkChaos(t *testing.T) {
    // Wrap actions with stochastic fault injection
    actions := testkit.WithChaos([]action.AnyAction{CreateOrder}, chaos.Config{
        Enabled:   true,
        ErrorRate: 0.25,                  // 25% simulated transient 503 errors
        MaxDelay:  150 * time.Millisecond, // Up to 150ms random network latency
        PanicRate: 0.02,                  // 2% unexpected panics to test recovery
    })

    suite := testkit.New(t, actions)

    // Run traffic to verify RetryMiddleware and PanicRecovery protect clients
    suite.POST("/v1/orders", map[string]any{"sku": "A"}).
        Do().
        ExpectSuccess()
}
```

---

## 12. Testing JSON-RPC / Stdio Transports

Use `testkit/rpc` for line-delimited JSON-RPC transports such as MCP stdio. It dials an in-memory `net.Pipe` and speaks JSON-RPC 2.0 exactly like a real client.

```go
import (
    "context"
    "io"
    "testing"

    "github.com/nexssp/testkit/rpc"
)

func TestMCP_Stdio(t *testing.T) {
    client := rpc.DialJSONRPC(t, func(ctx context.Context, in io.Reader, out io.Writer) error {
        return mcpServer.Serve(ctx, in, out)
    })

    resp := client.Call("tools/list", nil, 1)

    var data struct {
        Tools []struct {
            Name string `json:"name"`
        } `json:"tools"`
    }
    resp.BindResult(t, &data)

    // Assert with standard Go testing
    if len(data.Tools) != 1 {
        t.Fatalf("expected 1 tool, got %d", len(data.Tools))
    }
}
```

### `rpc.Client` API

```go
client := rpc.DialJSONRPC(t, serveFunc)

// Request / response
resp := client.Call("tools/list", nil, 1)

// Notification (no response)
_ = client.Notify("notifications/initialized", nil)

// Raw payload line (e.g. parse-error tests)
resp = client.CallRaw(`{"jsonrpc":"2.0","id":1,"method":"ping"}`)

// Typed result binding
resp.BindResult(t, &myStruct)

// JSON-RPC error object
if resp.Error != nil {
    t.Fatalf("RPC error: %+v", resp.Error)
}
```

---

## License

Apache License 2.0. See [LICENSE](LICENSE) for details.
