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

## Automatic main measurements

Pushes to `main` request central benchmarks for code, dependency, build, or
validation changes. Documentation-only and repository metadata changes are skipped
according to the [workflow filters](../.github/workflows/benchmark.yml).

Each request identifies the exact push SHA, and the central ledger preserves its
dependency selection and outcomes. For manual measurements and recovery, see the
[central workflow guide](https://github.com/qortoo/qortoo-bench/blob/main/docs/central-workflow.md).

PRs targeting `main` request a separate [candidate base/head comparison](https://github.com/qortoo/qortoo-bench/blob/main/docs/pr-candidates.md)
after the PR event workflow succeeds. Fork PRs follow the source repository's
Actions approval policy. The candidate result never updates official history or
website performance data and does not block merging based on a numeric threshold.
Manual central runs remain available. Central latency measurements do not measure
heap allocations.

The PR notifier locates open PRs by the source run's head owner and branch, then
requires an exact repository, branch, and commit match. Recovery of an old head
skips the request instead of substituting the PR's latest commit.
