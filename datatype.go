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
