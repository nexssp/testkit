# Contributing to Nexss Ecosystem

This repository is part of the **Nexss Ecosystem**.

We keep packages small, focused, and composable. Contributions should improve
correctness, clarity, portability, or measured performance without expanding the
public API unnecessarily.

## Toolchain

Every workflow in this repo goes through [Task](https://taskfile.dev). Do not
invoke `go build`, `go test`, `golangci-lint`, or `govulncheck` directly — the
`task` targets keep flags, ordering, and cache behavior identical to CI.

Install Task once:

```bash
brew install go-task/tap/go-task                       # macOS / Linuxbrew
scoop install task                                     # Windows (Scoop)
go install github.com/go-task/task/v3/cmd/task@latest  # any platform
```

Bootstrap the repo:

```bash
task           # list every available target
task setup     # fetch repo templates + install tool deps (gotestsum, govulncheck)
```

## Before opening a pull request

Run the full gate:

```bash
task tidy       # go mod tidy — keep go.mod / go.sum clean
task fmt        # gofumpt + gci (auto-fix)
task pre-push   # [fmt:check, lint, test:race]
task ci         # [fmt:check, lint, test:race:nocache] — exactly what CI runs
```

Optional extra checks when relevant:

```bash
task vuln       # govulncheck
task bench      # benchmark smoke test
task examples   # run curated .nflow files as runtime smoke checks
task lint:nflow # lint ./examples and ./spec
task self       # nflow self test
task test:smoke # CLI smoke test
```

Run `task examples`, `task lint:nflow`, or `task test:smoke` whenever you touch
the compiler, the DSL, `.nflow` files, or CLI wiring. A green `task test` is not
enough on its own in those cases.

## Requirements

- A change to a **public interface** requires a compatibility explanation and tests.
- A performance claim requires a benchmark on a documented Go version and hardware.
- New dependencies require a clear reason, license review, and evidence that the
  dependency is not better placed in an adapter module.
- Keep business rules, transports, hosted services, and provider-specific
  behavior outside this repository.
- New DSL features must use the existing extension/bundle mechanism — scaffold
  with `task ext:new -- <go_package_name>` rather than hand-rolling the layout.
- Do not add `testify` or other assertion helpers; use `xtest` / `xtest/ktest`.

## Local pre-commit (optional)

Install the hooks:

```bash
pre-commit install
# or
prek install
```

Hooks are wired to `task pre-commit` (on commit) and `task pre-push` (on push),
so **Task must be installed first** — see the Toolchain section above.

## Getting help

If a core or Kernel change appears necessary, write a short RFC before opening
the PR: describe the missing contract, alternatives considered, and why an
existing extension or adapter cannot solve the problem.

Thank you for contributing to Nexss.
