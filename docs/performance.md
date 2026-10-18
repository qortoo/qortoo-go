# Performance

[Qortoo Bench](https://github.com/qortoo/qortoo-bench) owns benchmark workloads,
measurement contracts, and official Rust/Go latency results. Run measurements with
its [local instructions](https://github.com/qortoo/qortoo-bench#run-full-measurements)
or [manual GitHub workflow](https://github.com/qortoo/qortoo-bench/blob/main/docs/central-workflow.md).
SDK functional tests and native SDK/ABI checks remain in their owning repositories.

The central source lock selects exact SDK commits. Local uncommitted SDK changes
are not benchmark inputs. Each result records Rust/Go revisions, native SDK
checksums, the harness, environment, and raw measurements. Workload or timing
changes belong to the central [Counter](https://github.com/qortoo/qortoo-bench/blob/main/docs/counter-measurement.md)
and [Variable](https://github.com/qortoo/qortoo-bench/blob/main/docs/variable-measurement.md) contracts.

## Interpret results

The central [result guide](https://github.com/qortoo/qortoo-bench/blob/main/docs/results.md)
and [metric/series contract](https://github.com/qortoo/qortoo-bench/blob/main/docs/result-publishing.md)
define comparable inputs and retained evidence. Latency differences include the
complete public API path, serialization, and runtime work; they are not pure cgo
overhead. Do not combine measurements from different host environments as a
binding-cost comparison.

## Go-only diagnostics

The SDK's CodSpeed workflow runs the five Go benchmarks in `benchmark_test.go`
against a release native SDK. These adaptive `testing.B` runs are Go-only
diagnostics, not the central fixed-budget Rust/Go comparison. Their Go allocation
counters cover the Go heap and do not include Rust native heap allocations.
The central latency harness does not measure allocations.
