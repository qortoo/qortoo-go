# Getting Started

`qortoo-go` wraps the `qortoo-ffi` static library with cgo. The Go module and the
native SDK are versioned together during the pre-1.0 period, so build this module
against the exact `qortoo-rs` revision pinned in `.github/workflows/ci.yml`.

## Requirements

- Go 1.25 or newer
- `CGO_ENABLED=1` and a C toolchain
- A native SDK containing `include/qortoo.h`, `lib/libqortoo_ffi.a`,
  `lib/pkgconfig/qortoo-ffi.pc`, and `manifest.json`

The supported CI targets are Linux and macOS on the architectures provided by
`ubuntu-latest` and `macos-latest`. The native library is linked statically, so built
programs do not need `LD_LIBRARY_PATH`, `DYLD_LIBRARY_PATH`, or a separately installed
Qortoo dynamic library at runtime.

## Prepare the Native SDK

There is no published native SDK release yet. Build one from a `qortoo-rs` checkout, at
the revision this module is actually tested against:

```shell
git -C /path/to/qortoo-rs checkout "$(grep -o 'QORTOO_RS_REF: [0-9a-f]*' .github/workflows/ci.yml | cut -d' ' -f2)"
make -C /path/to/qortoo-rs native-sdk-stage
export CGO_CFLAGS="-I/path/to/qortoo-rs/target/native-sdk/debug/include"
export CGO_LDFLAGS="-L/path/to/qortoo-rs/target/native-sdk/debug/lib"
```

The repositories do not need to be siblings. For development, `make test` stages the SDK
automatically from whatever revision the `qortoo-rs` checkout is currently on — it does
not check out `QORTOO_RS_REF` for you; set `QORTOO_RS_DIR` when that checkout is
elsewhere:

```shell
QORTOO_RS_DIR=/path/to/qortoo-rs make test
```

If an SDK is already installed, derive the flags from its pkg-config metadata:

```shell
export PKG_CONFIG_PATH="/path/to/native-sdk/lib/pkgconfig"
export CGO_CFLAGS="$(pkg-config --cflags qortoo-ffi)"
export CGO_LDFLAGS="$(pkg-config --libs qortoo-ffi)"
make test
```

At process startup, the package compares the linked library's ABI major with the ABI
major in `version.go`. A mismatch panics before the application can cross an unsafe ABI
boundary. The SDK version, ABI version, target, and build profile are also recorded in
the SDK's `manifest.json`.

That check is narrower than it may look. A matching ABI major only means the struct
layouts, symbol names, and calling conventions the two sides agree on haven't changed —
it says nothing about behavior. The only revision this module's own CI has actually built
and tested it against is the exact one in `QORTOO_RS_REF`; a different `qortoo-rs`
revision that happens to share the same ABI major passes the panic check but is
untested by this repository's own CI.

## Create and Update a Counter

```go
package main

import (
    "fmt"
    "log"

    "github.com/qortoo/qortoo-go"
)

func main() {
    client, err := qortoo.NewClient("my-collection", "my-client")
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
    fmt.Println(value)
}
```

## Store a JSON Value in a Variable

A `Variable` holds a single JSON value and resolves concurrent writes by
last-writer-wins, so every replica converges on the same winner regardless of the order
in which the writes arrive.

```go
variable, err := client.SubscribeOrCreateVariable("profile", nil)
if err != nil {
    log.Fatal(err)
}
defer variable.Close()

type Profile struct {
    Name string `json:"name"`
    Age  int    `json:"age"`
}

// Set returns the value held just before the call — nil before the first Set.
previous, err := variable.Set(Profile{Name: "ada", Age: 36})
if err != nil {
    log.Fatal(err)
}
fmt.Println(previous)

var profile Profile
if err := variable.Get(&profile); err != nil {
    log.Fatal(err)
}
```

`Set` marshals with `encoding/json` before any native call, so a value it cannot
marshal (a channel, a function, a cycle) is reported as the `encoding/json` error and
leaves the variable untouched. `Get` decodes into the destination pointer exactly as
`encoding/json` would.

The bytes are stored verbatim and are what a Rust or any other binding reads back, so
the value must be JSON-representable and both sides must agree on the schema. A
variable holds JSON null until the first `Set`, which is indistinguishable from an
explicitly stored `nil`; decode into a pointer or an `any` to tell null apart from a
stored zero value. Numbers reaching an `any` destination are kept exact as
`json.Number` instead of being rounded through `float64`. See
[Variable](https://github.com/qortoo/qortoo-rs/blob/main/docs/variable.md) in
`qortoo-rs` for the cross-language value contract.

Without a connectivity option, `NewClient` uses the no-op backend. For local
synchronization, share one `LocalConnectivity` between clients:

```go
connectivity := qortoo.NewLocalConnectivity()
defer connectivity.Close()

first, err := qortoo.NewClient("my-collection", "first",
    qortoo.WithLocalConnectivity(connectivity))
if err != nil {
    log.Fatal(err)
}
defer first.Close()
```

See [Lifecycle and Concurrency](lifecycle-and-concurrency.md) before using native
handles or callbacks in a long-running service.
