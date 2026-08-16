# Observability

The Rust core emits tracing spans and metrics but installs no process-global subscriber,
recorder, or exporter by default. A Go process opts in by calling `InitObservability`
once, before creating clients.

```go
err := qortoo.InitObservability(qortoo.ObservabilityOptions{
    ServiceName:       "checkout-service",
    LogFilter:         "qortoo=debug",
    LogFormat:         qortoo.LogFormatJSON,
    TraceEnabled:      true,
    OTLPEndpoint:      "http://localhost:4317",
    MetricsEnabled:    true,
    MetricsListenAddr: "0.0.0.0:9000",
})
if err != nil {
    return err
}
```

The zero-value options install nothing. An empty log filter uses `RUST_LOG`, then
`qortoo=info`; an empty OTLP endpoint uses
`OTEL_EXPORTER_OTLP_TRACES_ENDPOINT`, then `OTEL_EXPORTER_OTLP_ENDPOINT`, then
`http://localhost:4317`.

Initialization fails rather than silently dropping configured telemetry when it is
called twice, called after shutdown, conflicts with another Rust global subscriber or
metrics recorder, receives invalid options, or cannot start an exporter.

## Shutdown

`ShutdownObservability` flushes pending telemetry and stops the exporters. It uses the
remaining context deadline as its budget and defaults to five seconds when there is no
deadline. Calling it before initialization or calling it more than once is safe.
Shutdown is terminal because Rust process-global telemetry cannot be installed again.

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
if err := qortoo.ShutdownObservability(ctx); err != nil {
    log.Printf("flush qortoo telemetry: %v", err)
}
```

Close counters and clients before shutting down Qortoo observability, then shut down the
application's own telemetry provider. A blocked handler can delay both client and
observability shutdown.

## Trace Context

An OpenTelemetry `context.Context` cannot cross cgo directly. `SyncContext` and
`TransactionContext` inject W3C `traceparent` and `tracestate` strings; the FFI restores
them as a remote parent before entering the Rust core.

```go
ctx, span := tracer.Start(ctx, "checkout")
defer span.End()

if err := counter.SyncContext(ctx); err != nil {
    return err
}
```

The FFI sync span, the push/pull work on the event-loop thread, and handler callbacks
dispatched from that sync retain the caller's trace ID. A context without a valid span
behaves like `Sync` or `Transaction`. Context cancellation is not a cancellation signal
for these blocking native operations; it only supplies trace context.

## Runnable Examples

The programs under `examples/observability/` cover stdout logging, OTLP tracing,
Prometheus metrics, and Go profiling with Pyroscope. Their
[README](../examples/observability/README.md) documents the local Grafana stack and all
environment variables.

The Rust span and metric catalogues, and the internals of the FFI trace-context bridge,
remain documented in [`qortoo-rs`](https://github.com/qortoo/qortoo-rs/blob/main/docs/observability.md).
