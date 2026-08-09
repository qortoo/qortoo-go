# qortoo-go

Go binding for the [Qortoo](https://github.com/qortoo/qortoo-rs) CRDT SDK. It wraps
`qortoo-ffi`'s C ABI with cgo and exposes idiomatic Go types (`Client`, `Counter`,
`Handler`) over the same conflict-free counter datatype, transactions, and
observability pipeline as the Rust core.

Qortoo has not reached a first release yet: the public API, the native SDK
distribution format, and this package's own versioning are all still subject to
change without notice.

## Requirements

- Go 1.25 or newer (see `go.mod`).
- `CGO_ENABLED=1` and a C toolchain, since this package links `libqortoo_ffi.a` through
  cgo.
- A `qortoo-ffi` native SDK: the compiled static library, its C header, and pkg-config
  metadata, built from a [`qortoo-rs`](https://github.com/qortoo/qortoo-rs) checkout.
  There is no published native SDK release yet, so building one locally is currently
  the only option.

## Building and testing

With a `qortoo-rs` checkout next to this one (`../qortoo-rs`):

```sh
make test
```

This stages `qortoo-ffi`'s native SDK from `../qortoo-rs` automatically and points
`CGO_CFLAGS`/`CGO_LDFLAGS` at it. Use `QORTOO_RS_DIR=/path/to/qortoo-rs make test` for
a checkout in a different location.

Without a `qortoo-rs` checkout available — for example, against a native SDK installed
some other way — set `CGO_CFLAGS`/`CGO_LDFLAGS` yourself and `make test` uses them as
given, skipping auto-staging:

```sh
export CGO_CFLAGS="-I/path/to/native-sdk/include"
export CGO_LDFLAGS="-L/path/to/native-sdk/lib"
make test
```

`make test` is equivalent to `go vet ./...` and `go test -race ./...` once
`CGO_CFLAGS`/`CGO_LDFLAGS` are in place.

Importing this package checks the linked native library's ABI major version against
the version this package was built against (see `version.go`) and panics on a
mismatch, rather than risking undefined behavior across the FFI boundary.

## Usage

```go
package main

import (
    "fmt"
    "log"

    "github.com/qortoo/qortoo-go"
)

func main() {
    client, err := qortoo.NewClient("my-collection", "my-alias")
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    counter, err := client.SubscribeOrCreateCounter("visits", nil)
    if err != nil {
        log.Fatal(err)
    }
    defer counter.Close()

    value, err := counter.IncreaseBy(1)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println("counter value:", value)
}
```

Without a connectivity option, `NewClient` uses the SDK-default no-op backend, so this
example never leaves the process. See
[`qortoo.NewLocalConnectivity`](https://github.com/qortoo/qortoo-go/blob/main/qortoo.go)
for the in-memory backend used in tests, and
[`docs/go-binding.md`](https://github.com/qortoo/qortoo-rs/blob/main/docs/go-binding.md)
in `qortoo-rs` for the full binding architecture, error mapping, and observability
setup.

## Examples

`examples/observability/` contains runnable trace, log, metrics, and profiling
programs; see [its README](examples/observability/README.md) for setup against a local
Grafana stack.

## License

Apache License 2.0 — see [`LICENSE`](LICENSE).
