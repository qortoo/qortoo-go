# Performance

`benchmark_test.go` measures the Go binding. The Rust core owns its counterpart in
[`qortoo-rs/benches/qortoo_bench.rs`](https://github.com/qortoo/qortoo-rs/blob/main/benches/qortoo_bench.rs).
Both emit Go benchmark format with the same scenario names and fixed operation budgets,
so `benchstat -col /impl` can isolate the binding overhead.

## Scenarios

The executable contract lives in `benchmarks/contract.tsv`:

| Scenario | Budget per round | Workload |
| --- | ---: | --- |
| `IncreaseBy` | 200,000 | One counter write |
| `GetValue` | 2,000,000 | One counter read |
| `Transaction` | 20,000 | One transaction containing 10 writes |
| `SyncLocal` | 20,000 | One write and one manual push/pull |
| `BuildCounter` | 100 | Build and close one counter |

Every scenario runs for ten rounds. Mutable scenarios replace the datatype periodically
outside the measured region so accumulated pending operations and server history do not
put the Rust and Go halves into different cost regimes. `LocalConnectivity` runs in
manual mode, measurements are single-threaded, and no handler is registered.

The contract check verifies that every manifest scenario exists in both harnesses and
that its budget matches the Rust constant. It is run in regular and canary CI:

```shell
./scripts/check-benchmark-contract.sh \
  --qortoo-go /path/to/qortoo-go \
  --qortoo-rs /path/to/qortoo-rs
```

Changing a scenario, workload, reset interval, or budget requires coordinated changes
to both repositories and to `benchmarks/contract.tsv`.

## Run the Go Benchmarks

`make bench` builds a release native SDK and runs every scenario with its fixed budget:

```shell
QORTOO_RS_DIR=/path/to/qortoo-rs make bench
```

To use an already staged release SDK without a Rust checkout:

```shell
./scripts/run-go-benchmarks.sh \
  --qortoo-go /path/to/qortoo-go \
  --native-sdk /path/to/native-sdk
```

A debug `libqortoo_ffi.a` is rejected because it makes the comparison meaningless.

## Compare Rust and Go

The comparison command requires both checkout paths and a new output directory. It does
not assume sibling directory names or an execution order:

```shell
./scripts/compare-benchmarks.sh \
  --qortoo-go /path/to/qortoo-go \
  --qortoo-rs /path/to/qortoo-rs \
  --output /tmp/qortoo-benchmark-run
```

The output directory contains:

- `benchmarks.txt`: normalized Rust and Go samples in one Go benchmark file
- `metadata.txt`: both commit SHAs, native SDK and ABI versions, target, UTC time,
  hostname, OS, architecture, Rust version, and Go version
- `benchstat.txt`: the binding-overhead table when `benchstat` is installed

Install benchstat with `go install golang.org/x/perf/cmd/benchstat@latest`.

## Save and Compare Results

`make bench-save` runs the complete comparison and creates a timestamped directory under
the gitignored `benchmarks/results/`. Its name records both abbreviated commit SHAs and
the host; `metadata.txt` records the full SHAs, branches, and whether either checkout
was dirty.

```shell
QORTOO_RS_DIR=/path/to/qortoo-rs make bench-save
make bench-overhead                         # newest saved result
make bench-overhead RESULT=/path/to/result
make bench-compare BASE=/path/to/older      # NEW defaults to the newest result
make bench-compare BASE=/path/to/older NEW=/path/to/newer
```

`RESULT`, `BASE`, and `NEW` accept either a saved result directory or its
`benchmarks.txt`. `FILE` remains an alias for `RESULT` for compatibility with the
original benchmark targets. Cross-run comparison rejects results whose metadata names
different hosts.

Compare only runs from the same host. Wall-clock noise can dominate `SyncLocal` and
`BuildCounter`, while `B/op` and `allocs/op` are usually more stable.

## Upload to Bencher

Install and authenticate the Bencher CLI, then upload the newest result:

```shell
BENCHER_API_KEY=bencher_run_... make bench-upload
```

Select an older result or override the default project when necessary:

```shell
BENCHER_API_KEY=bencher_run_... \
  make bench-upload RESULT=/path/to/result BENCHER_PROJECT=qortoo-sync
```

The default testbed comes from the host recorded in `metadata.txt`, and the report is
attached to the recorded `qortoo-go` branch and commit SHA. `BENCHER_TESTBED` overrides
the testbed.
The credential is read only from `BENCHER_API_KEY` (preferred) or the legacy
`BENCHER_API_TOKEN` and is never written to the result directory.

Validate parsing without sending a report:

```shell
BENCHER_DRY_RUN=1 make bench-upload RESULT=/path/to/result
```
