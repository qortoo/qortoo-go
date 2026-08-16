# Repository Guidelines

## Project Overview

qortoo-go is the Go binding of the [Qortoo](https://github.com/qortoo/qortoo-rs) CRDT
SDK. It wraps `qortoo-ffi`'s C ABI with cgo and exposes idiomatic Go types (`Client`,
`Counter`, `Handler`) over the Rust core's counter datatype, transactions, and
observability pipeline. Connectivity backends, the C ABI itself, and native SDK
releases are owned by `qortoo-rs`; this repository owns only the Go public API, the
cgo adapter, and Go-side tests, examples, and documentation.

## Critical guidelines

Shared across all qortoo-* repos (canonical text in [qortoo-harness `AGENTS.md`](https://github.com/qortoo/qortoo-harness)):
- All code comments and documentation must be written in **English**.
- Favor SOLID principles, especially Single Responsibility (SRP) and Open/Closed (OCP), where practical. Check for these during review.
- Individual plans, task lists, and working notes belong in the gitignored `.local/` at the repo root — never commit them or propose committing them. Claude Code's project-scoped agent memory (`.claude/agent-memory/`) is likewise personal and gitignored, not shared team knowledge.

## Project Structure & Module Organization

- Root `.go` files hold the package: `qortoo.go` (`Client`, `LocalConnectivity`),
  `counter.go` (`Counter`, transactions, `Handler`), `callbacks.go` (cgo callback
  trampolines), `errors.go`, `lifecycle.go`, `observability.go`, `version.go` (ABI
  version check), and `cgo.go` (cgo preamble and linker flags).
- `examples/observability/` contains runnable trace, log, metrics, and profiling
  programs against a local Grafana stack (see its own README).
- `docs/` owns installation, Go API lifecycle/concurrency, Go observability, and
  binding-performance guidance.
- `benchmark_test.go` mirrors the scenario names, workloads, and iteration budgets of
  `benches/qortoo_bench.rs` in `qortoo-rs`. `benchmarks/contract.tsv` and
  `scripts/check-benchmark-contract.sh` keep both sides comparable.

## Go Toolchain

- **Minimum Go version**: see the `go` directive in `go.mod`.
- **CGO_ENABLED=1** is required; this package cannot build with cgo disabled.

## Native SDK Dependency

This package links `libqortoo_ffi.a`, `qortoo-ffi`'s C ABI native library, through
`CGO_CFLAGS`/`CGO_LDFLAGS` — it carries no default include or library search path (see
`cgo.go`). There is no published native SDK release yet, so `make test` stages a debug
SDK and `make bench` stages a release SDK from a `qortoo-rs` checkout
(`QORTOO_RS_DIR` overrides its location). See the
[Getting Started guide](docs/getting-started.md) for details.

Importing this package checks the linked library's ABI major version
(`qortoo_abi_version_major`) against the version this package is built against
(`version.go`) and panics on a mismatch, rather than risking undefined behavior.

## Build, Test, and Development Commands

```shell
# Stages the native SDK from ../qortoo-rs (or CGO_CFLAGS/CGO_LDFLAGS, if already set)
# and runs go vet + go test -race; see README.md#building-and-testing.
make test

# Equivalent, once CGO_CFLAGS/CGO_LDFLAGS are in place:
go vet ./...
go test -race ./...

# Fixed-budget Go benchmarks linked to a release native SDK
QORTOO_RS_DIR=/path/to/qortoo-rs make bench

# Cross-language comparison; both checkout paths are explicit
./scripts/compare-benchmarks.sh --qortoo-go /path/to/qortoo-go \
  --qortoo-rs /path/to/qortoo-rs --output /tmp/qortoo-benchmark-run

# Save, inspect, compare, and upload cross-language results
QORTOO_RS_DIR=/path/to/qortoo-rs make bench-save
make bench-overhead
make bench-compare BASE=/path/to/older-result
BENCHER_API_KEY=... make bench-upload
```

## Coding Style & Naming Conventions

- Standard Go style: `gofmt`-formatted, `CamelCase` for exported identifiers,
  `camelCase` for unexported ones.
- Go names mirror the Rust core's names 1:1 (e.g., `LocalConnectivity` wraps the Rust
  type of the same name), so navigating between this binding and `qortoo-rs` requires
  no mental mapping.
- Userdata crossing the cgo boundary travels as `uintptr_t`, never `void*`: Go's
  `checkptr` (enabled by `-race`) rejects integer-to-pointer round trips of
  `cgo.Handle`.

## Testing Guidelines

- `go test -race ./...` is the baseline; observability tests additionally run with
  `-shuffle=on -count=2` in CI because installing a global subscriber/recorder is only
  safe once per process (see `observability_test.go`).
- Every object wraps a native handle and must be released with `Close()`. Tests that
  create a `Client`/`Counter`/`LocalConnectivity` must `defer` its `Close()`.

## Documentation Guidelines

Shared across all qortoo-* repos (canonical text in [qortoo-harness `AGENTS.md`](https://github.com/qortoo/qortoo-harness)):
- Qortoo has not been released yet, so documentation should describe only the current intended behavior.
- Do not add development-history or migration notes for intermediate, unreleased changes (for example, that a field or label previously existed and was removed) unless the user explicitly asks for that history.
- Add compatibility or migration guidance only for behavior that has appeared in a released version.
- Docs describe the system as it is now and the rule that holds it in place — write principles, not history.

Start at [`docs/README.md`](docs/README.md) for Go-owned guides. For the C ABI, error
mapping, callback/userdata ownership, and native SDK release process, see
[`docs/go-binding.md`](https://github.com/qortoo/qortoo-rs/blob/main/docs/go-binding.md)
in `qortoo-rs` — this repository does not duplicate that documentation.

## Commit & Pull Request Guidelines

Shared across all qortoo-* repos (canonical text in [qortoo-harness `AGENTS.md`](https://github.com/qortoo/qortoo-harness); automated via the `qortoo-shared` plugin's `/qortoo-shared:commit` and `/qortoo-shared:pr`):
- Commit messages follow an emoji + conventional format: `🧬feat(scope): summary`, `📜docs(scope): summary`, `💿CI/CD(scope): summary`.
- Keep scopes specific to the module (e.g. `counter`, `observability`, `examples`).
- PRs should include a concise summary, testing notes (commands run), and links to issues if applicable.
- If behavior changes, update docs and add/adjust tests.
