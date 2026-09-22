# testkit examples

Every file in this directory is a runnable test that demonstrates one
feature of `github.com/nexssp/testkit` in isolation.

Run all of them:

    go test ./testkit/examples/... -v

Run one:

    go test ./testkit/examples/trace/... -v

Each example is deliberately short. If you can read the test, you can
use the feature. If a test is longer than the feature description,
it is a bug in the test.

## Index

| Directory        | Feature                                            |
|------------------|----------------------------------------------------|
| `01_http_suite`  | Fluent HTTP suite against an `http.Handler`        |
| `02_eventually`  | Polling for async completion                        |
| `03_trace`       | Recording and asserting on the action call sequence |
| `04_simulate`    | Thundering herd / barrier-based concurrency test    |
| `05_contracts`   | Structural invariants across every action           |
| `06_chaos`       | Fault injection middleware                          |
| `07_golden`      | Golden-file snapshot (kernel `xtest`, `-xtest.update`) |
