package qortoo

/*
#include <qortoo.h>

// Fn-pointer getters defined in cgo.go's preamble (external linkage).
extern QortooOnStateChangeCallback qortooGoStateChangeCB(void);
extern QortooOnErrorCallback qortooGoErrorCB(void);
extern QortooUserdataDropCallback qortooGoUserdataDropCB(void);
*/
import "C"

import (
	"context"
	"fmt"
	"runtime"
	"runtime/cgo"
)

// Handler receives datatype notifications. Both fields are optional.
// Callbacks run on Qortoo-owned worker threads (see package docs).
type Handler struct {
	// OnStateChange is called as (oldState, newState) after a lifecycle transition.
	OnStateChange func(oldState, newState DatatypeState)
	// OnError is called with errors surfaced from the sync path.
	OnError func(err *Error)
}

// DatatypeOptions configures datatype construction. The zero value (or nil
// pointer) selects the SDK defaults.
type DatatypeOptions struct {
	// Readonly rejects all local writes regardless of state.
	Readonly bool
	// MaxPushBufferSize in bytes; 0 keeps the SDK default (clamped by the SDK).
	MaxPushBufferSize uint64
	// Handler registered at build time (needed to observe the first auto-sync
	// with realtime connectivity). HandlerPriority orders multiple handlers.
	Handler         *Handler
	HandlerPriority uint
}

func (opts *DatatypeOptions) toC() *C.QortooDatatypeOptions {
	if opts == nil {
		return nil
	}
	c := &C.QortooDatatypeOptions{
		readonly:             C.bool(opts.Readonly),
		max_push_buffer_size: C.uint64_t(opts.MaxPushBufferSize),
		handler_priority:     C.uintptr_t(opts.HandlerPriority),
	}
	if opts.Handler != nil {
		c.on_state_change = C.qortooGoStateChangeCB()
		c.on_error = C.qortooGoErrorCB()
		c.handler_userdata = newHandlerUserdata(opts.Handler)
		c.handler_userdata_drop = C.qortooGoUserdataDropCB()
	}
	return c
}

// newHandlerUserdata addresses h for the duration of a handler registration.
// Rust fires userdata_drop exactly once — when the handler is replaced, unset,
// or could not be registered at all — and that is what releases the handle.
func newHandlerUserdata(h *Handler) C.uintptr_t {
	return C.uintptr_t(cgo.NewHandle(h))
}

// datatype is the state and behavior every datatype shares. A datatype embeds it,
// which promotes these methods onto the exported type — godoc renders them on the
// datatype itself — so each datatype file adds only its own operations.
//
// shared is the datatype's own handle viewed as the shared one every
// qortoo_datatype_* entry point takes. It borrows the concrete handle: the two point
// into the same native allocation, which the datatype's own free function releases.
type datatype struct {
	shared *C.QortooDatatype
	// client keeps the owning Client reachable so its GC cleanup cannot shut the
	// native client down while this datatype is still in use. Severed by Close;
	// nil for transaction-scoped handles.
	client *Client
	// borrowed marks transaction-scoped handles owned by Rust (must not be freed).
	borrowed bool
	cleanup  runtime.Cleanup
}

// release stops the GC cleanup and reports whether the caller still owns the native
// handle and must release it. A transaction-scoped handle is owned by Rust, and an
// already-closed one reports false — which is what makes Close idempotent. The
// caller clears its own concrete handle afterwards, so a later call reaches the
// FFI's null checks instead of freed memory.
func (d *datatype) release() bool {
	owned := d.shared != nil && !d.borrowed
	if owned {
		d.cleanup.Stop()
	}
	d.shared = nil
	d.client = nil
	return owned
}

// Sync performs a blocking push/pull with the connectivity backend. Use
// SyncContext to keep the sync inside the trace of a calling span.
func (d *datatype) Sync() error {
	defer runtime.KeepAlive(d)
	var cerr C.QortooError
	C.qortoo_datatype_sync(d.shared, &cerr)
	return takeError(&cerr)
}

// SyncContext is Sync continuing the trace of ctx.
//
// The Rust spans of this sync — including the push/pull that runs on a worker thread
// and the handler callbacks it dispatches — become children of the span in ctx.
// Without a span in ctx it behaves exactly like Sync. Cancellation of ctx is not
// honoured: the underlying sync is a blocking call.
func (d *datatype) SyncContext(ctx context.Context) error {
	defer runtime.KeepAlive(d)
	var cerr C.QortooError
	withTraceContext(ctx, func(traceparent, tracestate *C.char) {
		C.qortoo_datatype_sync_with_context(d.shared, traceparent, tracestate, &cerr)
	})
	return takeError(&cerr)
}

// Unsubscribe marks this datatype as unsubscribing (see Client.UnsubscribeDatatype).
func (d *datatype) Unsubscribe() error {
	defer runtime.KeepAlive(d)
	var cerr C.QortooError
	C.qortoo_datatype_unsubscribe(d.shared, &cerr)
	return takeError(&cerr)
}

// Key returns the datatype key.
func (d *datatype) Key() string {
	defer runtime.KeepAlive(d)
	return goString(C.qortoo_datatype_get_key(d.shared))
}

// Type returns the datatype kind, which is fixed by the concrete datatype
// (TypeCounter for a Counter).
func (d *datatype) Type() DataType {
	defer runtime.KeepAlive(d)
	return DataType(C.qortoo_datatype_get_type(d.shared))
}

// State returns the current lifecycle state.
func (d *datatype) State() DatatypeState {
	defer runtime.KeepAlive(d)
	return DatatypeState(C.qortoo_datatype_get_state(d.shared))
}

// ServerVersion returns the server-side version (0 before the first sync).
func (d *datatype) ServerVersion() uint64 {
	defer runtime.KeepAlive(d)
	return uint64(C.qortoo_datatype_get_server_version(d.shared))
}

// ClientVersion returns the number of local operations.
func (d *datatype) ClientVersion() uint64 {
	defer runtime.KeepAlive(d)
	return uint64(C.qortoo_datatype_get_client_version(d.shared))
}

// SyncedClientVersion returns the last client version acknowledged by the server.
func (d *datatype) SyncedClientVersion() uint64 {
	defer runtime.KeepAlive(d)
	return uint64(C.qortoo_datatype_get_synced_client_version(d.shared))
}

// SetHandler registers (or replaces) a handler at the given priority
// (lower priority runs first).
func (d *datatype) SetHandler(priority uint, h *Handler) {
	defer runtime.KeepAlive(d)
	C.qortoo_datatype_set_handler(
		d.shared,
		C.uintptr_t(priority),
		C.qortooGoStateChangeCB(),
		C.qortooGoErrorCB(),
		newHandlerUserdata(h),
		C.qortooGoUserdataDropCB(),
	)
}

// UnsetHandler removes the handler at the given priority. Returns true if one
// was removed.
func (d *datatype) UnsetHandler(priority uint) bool {
	defer runtime.KeepAlive(d)
	return bool(C.qortoo_datatype_unset_handler(d.shared, C.uintptr_t(priority)))
}

// buildDatatype runs the construction flow every datatype shares: it keeps the
// client alive for the call, marshals the key, converts the options, and reports
// the FFI error. build performs the datatype's own construction call and stores
// the resulting handle; the caller wraps it and registers its GC cleanup.
//
// The handle is passed out through build rather than returned here because the
// generated datatype handle types are incomplete (opaque C structs), so they
// cannot be used as type arguments.
func buildDatatype(
	c *Client,
	key string,
	opts *DatatypeOptions,
	build func(cl *C.QortooClient, k *C.char, o *C.QortooDatatypeOptions, e *C.QortooError),
) error {
	defer runtime.KeepAlive(c)
	cKey := cString(key)
	defer freeCString(cKey)
	var cerr C.QortooError
	build(c.ptr, cKey, opts.toC(), &cerr)
	return takeError(&cerr)
}

// txContext carries a transaction body and its error across the FFI boundary.
// T is the datatype handle type the body receives.
type txContext[T any] struct {
	fn  func(tx *T) error
	err error
}

// run executes the body with the borrowed transaction handle and returns the
// commit (0) or roll-back (non-zero) code the FFI expects. A panic must not
// unwind into Rust, so it is recovered and reported as a roll-back.
func (txc *txContext[T]) run(tx *T) (ret C.int32_t) {
	defer func() {
		if r := recover(); r != nil {
			txc.err = fmt.Errorf("qortoo: transaction panicked: %v", r)
			ret = 1
		}
	}()
	if err := txc.fn(tx); err != nil {
		txc.err = err
		return 1
	}
	return 0
}

// result prefers the error returned by the transaction body: the FFI only reports
// that the commit was aborted, while txc.err says why.
func (txc *txContext[T]) result(cerr *C.QortooError) error {
	err := takeError(cerr)
	if err != nil && txc.err != nil {
		return txc.err
	}
	return err
}

// runTransaction wires a transaction body through the FFI: it hands invoke the
// marshalled tag and the userdata addressing the body, then reports the outcome.
// invoke performs the datatype's own transaction call.
func runTransaction[T any](
	tag string,
	fn func(tx *T) error,
	invoke func(cTag *C.char, userdata C.uintptr_t, cerr *C.QortooError),
) error {
	cTag := cString(tag)
	defer freeCString(cTag)

	txc := &txContext[T]{fn: fn}
	h := cgo.NewHandle(txc)
	defer h.Delete()

	var cerr C.QortooError
	invoke(cTag, C.uintptr_t(h), &cerr)
	return txc.result(&cerr)
}
