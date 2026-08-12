# qortoo-go Documentation

This repository owns the Go API and its cgo adapter. The Rust core, C ABI, generated
header, and native SDK releases are maintained in
[`qortoo-rs`](https://github.com/qortoo/qortoo-rs).

| Document | Description |
| --- | --- |
| [Getting Started](getting-started.md) | Native SDK setup, build requirements, and the first counter |
| [Lifecycle and Concurrency](lifecycle-and-concurrency.md) | `Close`, cleanup fallback, callback threading, and transaction lifetime |
| [Observability](observability.md) | Logging, tracing, metrics, shutdown, and W3C trace-context propagation |
| [Performance](performance.md) | Go benchmarks and cross-repository Rust/Go comparison workflow |

The C ABI contract, error mapping, callback/userdata ownership, header generation, and
native SDK staging are documented by
[`qortoo-rs`](https://github.com/qortoo/qortoo-rs/blob/main/docs/go-binding.md).
