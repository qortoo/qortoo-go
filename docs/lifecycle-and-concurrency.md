# Lifecycle and Concurrency

Every `Client`, `Counter`, and `LocalConnectivity` owns a native handle. Call `Close`
deterministically; the runtime cleanup registered by each constructor is only a leak
fallback and is not guaranteed to run, including at process exit.

## Ownership and Close Order

- A `Counter` keeps its owning `Client` reachable until the counter is closed.
- Closing a `Client` shuts down its worker runtime. Existing counters remain safe to
  inspect and close, but they can no longer synchronize.
- A client built from a `LocalConnectivity` keeps its own native reference. Closing the
  Go connectivity wrapper does not invalidate those clients.
- `Close` is idempotent, but it must not run concurrently with another method on the
  same object.

For normal application shutdown, close counters before clients and close the shared
connectivity after its clients:

```go
counter.Close()
client.Close()
connectivity.Close()
```

When observability is enabled, the full order is:

```text
Counter.Close -> Client.Close -> LocalConnectivity.Close
              -> ShutdownObservability -> application telemetry shutdown
```

## Callback Concurrency

`Handler.OnStateChange` and `Handler.OnError` run asynchronously on Qortoo-owned Rust
worker threads. A handler may be entered concurrently, so its captured state must be
synchronized. It should also return promptly: blocking a handler delays later
notifications, `Client.Close`, and observability shutdown.

```go
var mu sync.Mutex
states := make([]qortoo.DatatypeState, 0, 4)

handler := &qortoo.Handler{
    OnStateChange: func(_, next qortoo.DatatypeState) {
        mu.Lock()
        states = append(states, next)
        mu.Unlock()
    },
}
```

Register a build-time handler through `DatatypeOptions` when it must observe the first
automatic synchronization. `SetHandler` replaces the handler at the same priority;
`UnsetHandler` removes it. Handler userdata is released exactly once when Rust drops the
registration, including failed registration paths.

## Transactions

The function passed to `Transaction` or `TransactionContext` runs inline on the calling
goroutine. Its `tx` counter is borrowed from Rust and is valid only for the duration of
that function. Do not retain it, pass it to another goroutine, or call `Close` on it.
Returning an error or panicking rolls back the operations performed through `tx`; a Go
panic is recovered before it can unwind across the FFI boundary.

Apart from concurrent `Close` on the same object, public operations are safe for
concurrent use. Application-level ordering still matters: a callback is asynchronous,
while a transaction body is synchronous.
