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

There is no published native SDK release yet. Build one from a `qortoo-rs` checkout:

```shell
make -C /path/to/qortoo-rs native-sdk-stage
export CGO_CFLAGS="-I/path/to/qortoo-rs/target/native-sdk/debug/include"
export CGO_LDFLAGS="-L/path/to/qortoo-rs/target/native-sdk/debug/lib"
```

The repositories do not need to be siblings. For development, `make test` stages the
SDK automatically; set `QORTOO_RS_DIR` when the Rust checkout is elsewhere:

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
