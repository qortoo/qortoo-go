package qortoo

/*
#include <qortoo.h>

// Fn-pointer getters defined in cgo.go's preamble (external linkage).
extern QortooOnStateChangeCallback qortooGoStateChangeCB(void);
extern QortooOnErrorCallback qortooGoErrorCB(void);
extern QortooUserdataDropCallback qortooGoUserdataDropCB(void);
extern QortooTxCallback qortooGoTxCB(void);
*/
import "C"

import (
	"context"
	"runtime"
)

// Counter is a conflict-free counter datatype.
type Counter struct {
	ptr *C.QortooCounter
	// client keeps the owning Client reachable so its GC cleanup cannot shut
	// the native client down while this counter is still in use. Severed by
	// Close; nil for transaction-scoped handles.
	client *Client
	// borrowed marks transaction-scoped handles owned by Rust (must not be freed).
	borrowed bool
	cleanup  runtime.Cleanup
}

func (c *Client) buildCounter(
	key string,
	opts *DatatypeOptions,
	build func(*C.QortooClient, *C.char, *C.QortooDatatypeOptions, *C.QortooError) *C.QortooCounter,
) (*Counter, error) {
	var ptr *C.QortooCounter
	err := buildDatatype(c, key, opts, func(cl *C.QortooClient, k *C.char, o *C.QortooDatatypeOptions, e *C.QortooError) {
		ptr = build(cl, k, o, e)
	})
	if err != nil {
		return nil, err
	}
	ctr := &Counter{ptr: ptr, client: c}
	ctr.cleanup = runtime.AddCleanup(ctr, freeCounterPtr, ptr)
	return ctr, nil
}

// CreateCounter builds a counter in StateCreating (writable).
func (c *Client) CreateCounter(key string, opts *DatatypeOptions) (*Counter, error) {
	return c.buildCounter(key, opts, func(cl *C.QortooClient, k *C.char, o *C.QortooDatatypeOptions, e *C.QortooError) *C.QortooCounter {
		return C.qortoo_counter_create(cl, k, o, e)
	})
}

// SubscribeCounter builds a counter in StateSubscribing (read-only until synced).
func (c *Client) SubscribeCounter(key string, opts *DatatypeOptions) (*Counter, error) {
	return c.buildCounter(key, opts, func(cl *C.QortooClient, k *C.char, o *C.QortooDatatypeOptions, e *C.QortooError) *C.QortooCounter {
		return C.qortoo_counter_subscribe(cl, k, o, e)
	})
}

// SubscribeOrCreateCounter builds a counter in StateSubscribingOrCreating (writable).
func (c *Client) SubscribeOrCreateCounter(key string, opts *DatatypeOptions) (*Counter, error) {
	return c.buildCounter(key, opts, func(cl *C.QortooClient, k *C.char, o *C.QortooDatatypeOptions, e *C.QortooError) *C.QortooCounter {
		return C.qortoo_counter_subscribe_or_create(cl, k, o, e)
	})
}

// IncreaseBy adds delta (which may be negative) and returns the new value.
func (c *Counter) IncreaseBy(delta int64) (int64, error) {
	defer runtime.KeepAlive(c)
	var cerr C.QortooError
	v := C.qortoo_counter_increase_by(c.ptr, C.int64_t(delta), &cerr)
	if err := takeError(&cerr); err != nil {
		return 0, err
	}
	return int64(v), nil
}

// Increase adds 1 and returns the new value.
func (c *Counter) Increase() (int64, error) {
	return c.IncreaseBy(1)
}

// Value returns the current counter value.
func (c *Counter) Value() int64 {
	defer runtime.KeepAlive(c)
	return int64(C.qortoo_counter_get_value(c.ptr))
}

// Sync performs a blocking push/pull with the connectivity backend. Use
// SyncContext to keep the sync inside the trace of a calling span.
func (c *Counter) Sync() error {
	defer runtime.KeepAlive(c)
	var cerr C.QortooError
	C.qortoo_counter_sync(c.ptr, &cerr)
	return takeError(&cerr)
}

// SyncContext is Sync continuing the trace of ctx.
//
// The Rust spans of this sync — including the push/pull that runs on a worker thread
// and the handler callbacks it dispatches — become children of the span in ctx.
// Without a span in ctx it behaves exactly like Sync. Cancellation of ctx is not
// honoured: the underlying sync is a blocking call.
func (c *Counter) SyncContext(ctx context.Context) error {
	defer runtime.KeepAlive(c)
	var cerr C.QortooError
	withTraceContext(ctx, func(traceparent, tracestate *C.char) {
		C.qortoo_counter_sync_with_context(c.ptr, traceparent, tracestate, &cerr)
	})
	return takeError(&cerr)
}

// Unsubscribe marks this datatype as unsubscribing (see Client.UnsubscribeDatatype).
func (c *Counter) Unsubscribe() error {
	defer runtime.KeepAlive(c)
	var cerr C.QortooError
	C.qortoo_counter_unsubscribe(c.ptr, &cerr)
	return takeError(&cerr)
}

// Key returns the datatype key.
func (c *Counter) Key() string {
	defer runtime.KeepAlive(c)
	return goString(C.qortoo_counter_get_key(c.ptr))
}

// Type returns the datatype kind (always TypeCounter for a Counter).
func (c *Counter) Type() DataType {
	defer runtime.KeepAlive(c)
	return DataType(C.qortoo_counter_get_type(c.ptr))
}

// State returns the current lifecycle state.
func (c *Counter) State() DatatypeState {
	defer runtime.KeepAlive(c)
	return DatatypeState(C.qortoo_counter_get_state(c.ptr))
}

// ServerVersion returns the server-side version (0 before the first sync).
func (c *Counter) ServerVersion() uint64 {
	defer runtime.KeepAlive(c)
	return uint64(C.qortoo_counter_get_server_version(c.ptr))
}

// ClientVersion returns the number of local operations.
func (c *Counter) ClientVersion() uint64 {
	defer runtime.KeepAlive(c)
	return uint64(C.qortoo_counter_get_client_version(c.ptr))
}

// SyncedClientVersion returns the last client version acknowledged by the server.
func (c *Counter) SyncedClientVersion() uint64 {
	defer runtime.KeepAlive(c)
	return uint64(C.qortoo_counter_get_synced_client_version(c.ptr))
}

// SetHandler registers (or replaces) a handler at the given priority
// (lower priority runs first).
func (c *Counter) SetHandler(priority uint, h *Handler) {
	defer runtime.KeepAlive(c)
	C.qortoo_counter_set_handler(
		c.ptr,
		C.uintptr_t(priority),
		C.qortooGoStateChangeCB(),
		C.qortooGoErrorCB(),
		newHandlerUserdata(h),
		C.qortooGoUserdataDropCB(),
	)
}

// UnsetHandler removes the handler at the given priority. Returns true if one
// was removed.
func (c *Counter) UnsetHandler(priority uint) bool {
	defer runtime.KeepAlive(c)
	return bool(C.qortoo_counter_unset_handler(c.ptr, C.uintptr_t(priority)))
}

// Transaction executes fn atomically: if fn returns an error (or panics), every
// operation performed through tx is rolled back. fn runs inline on the calling
// goroutine; tx is only valid during the call. Use TransactionContext to keep
// the commit inside the trace of a calling span.
func (c *Counter) Transaction(tag string, fn func(tx *Counter) error) error {
	defer runtime.KeepAlive(c)
	return runTransaction(tag, fn, func(cTag *C.char, userdata C.uintptr_t, cerr *C.QortooError) {
		C.qortoo_counter_transaction(c.ptr, cTag, C.qortooGoTxCB(), userdata, cerr)
	})
}

// TransactionContext is Transaction continuing the trace of ctx: the commit and any
// sync it triggers become children of the span in ctx. Without a span in ctx it
// behaves exactly like Transaction. Cancellation of ctx is not honoured — fn runs
// inline on the calling goroutine.
func (c *Counter) TransactionContext(ctx context.Context, tag string, fn func(tx *Counter) error) error {
	defer runtime.KeepAlive(c)
	return runTransaction(tag, fn, func(cTag *C.char, userdata C.uintptr_t, cerr *C.QortooError) {
		withTraceContext(ctx, func(traceparent, tracestate *C.char) {
			C.qortoo_counter_transaction_with_context(
				c.ptr, cTag, traceparent, tracestate, C.qortooGoTxCB(), userdata, cerr)
		})
	})
}

// Close releases this handle. The underlying datatype stays registered in its
// client. No-op for transaction-scoped handles. Close must not be called
// concurrently with other methods on the same object.
func (c *Counter) Close() {
	if c.ptr != nil && !c.borrowed {
		c.cleanup.Stop()
		C.qortoo_counter_free(c.ptr)
	}
	c.ptr = nil
	c.client = nil
}
