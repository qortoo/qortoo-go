package qortoo

/*
#include <qortoo.h>

// Fn-pointer getters defined in cgo.go's preamble (external linkage).
extern QortooOnStateChangeCallback qortooGoStateChangeCB(void);
extern QortooOnErrorCallback qortooGoErrorCB(void);
extern QortooUserdataDropCallback qortooGoUserdataDropCB(void);
extern QortooVariableTxCallback qortooGoVariableTxCB(void);
*/
import "C"

import (
	"bytes"
	"context"
	"encoding/json"
	"runtime"
	"unsafe"
)

// Variable is a last-writer-wins datatype holding a single JSON value.
//
// A variable always holds a value; before the first Set that value is JSON null,
// which is indistinguishable from an explicitly stored nil. Concurrent writes are
// resolved by the logical timestamp of each Set, so every replica converges on the
// same winner regardless of the order in which the writes arrive.
//
// Values cross the boundary as JSON produced by encoding/json, which is also what a
// Rust or any other binding reads back — so the value must be JSON-representable
// (channels, functions, cycles, and non-string map keys are not) and both sides must
// agree on the schema. The stored bytes are kept verbatim: they are never reparsed,
// reordered, or re-encoded by the SDK.
type Variable struct {
	datatype
	ptr *C.QortooVariable
}

func (c *Client) buildVariable(
	key string,
	opts *DatatypeOptions,
	build func(*C.QortooClient, *C.char, *C.QortooDatatypeOptions, *C.QortooError) *C.QortooVariable,
) (*Variable, error) {
	var ptr *C.QortooVariable
	err := buildDatatype(c, key, opts, func(cl *C.QortooClient, k *C.char, o *C.QortooDatatypeOptions, e *C.QortooError) {
		ptr = build(cl, k, o, e)
	})
	if err != nil {
		return nil, err
	}
	v := &Variable{
		datatype: datatype{shared: C.qortoo_variable_as_datatype(ptr), client: c},
		ptr:      ptr,
	}
	v.cleanup = runtime.AddCleanup(v, freeVariablePtr, ptr)
	return v, nil
}

// CreateVariable builds a variable in StateCreating (writable).
func (c *Client) CreateVariable(key string, opts *DatatypeOptions) (*Variable, error) {
	return c.buildVariable(key, opts, func(cl *C.QortooClient, k *C.char, o *C.QortooDatatypeOptions, e *C.QortooError) *C.QortooVariable {
		return C.qortoo_variable_create(cl, k, o, e)
	})
}

// SubscribeVariable builds a variable in StateSubscribing (read-only until synced).
func (c *Client) SubscribeVariable(key string, opts *DatatypeOptions) (*Variable, error) {
	return c.buildVariable(key, opts, func(cl *C.QortooClient, k *C.char, o *C.QortooDatatypeOptions, e *C.QortooError) *C.QortooVariable {
		return C.qortoo_variable_subscribe(cl, k, o, e)
	})
}

// SubscribeOrCreateVariable builds a variable in StateSubscribingOrCreating (writable).
func (c *Client) SubscribeOrCreateVariable(key string, opts *DatatypeOptions) (*Variable, error) {
	return c.buildVariable(key, opts, func(cl *C.QortooClient, k *C.char, o *C.QortooDatatypeOptions, e *C.QortooError) *C.QortooVariable {
		return C.qortoo_variable_subscribe_or_create(cl, k, o, e)
	})
}

// Set stores value as the new JSON value and returns the value held just before
// this call — nil for the first Set and for a stored JSON null.
//
// The previous value has no destination type to decode into (a variable may hold a
// different shape on every Set), so it is returned dynamically, with JSON numbers
// kept as json.Number.
//
// value is marshalled with encoding/json before any native call: a marshalling
// error is returned as-is and leaves the variable untouched.
func (v *Variable) Set(value any) (any, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	defer runtime.KeepAlive(v)

	var previous C.QortooOwnedBytes
	var cerr C.QortooError
	// The native call copies data before returning, so passing Go memory is safe.
	C.qortoo_variable_set_raw(
		v.ptr,
		(*C.uint8_t)(unsafe.Pointer(unsafe.SliceData(data))),
		C.uintptr_t(len(data)),
		&previous,
		&cerr,
	)
	raw := takeOwnedBytes(&previous)
	if err := takeError(&cerr); err != nil {
		return nil, err
	}
	return decodeJSONValue(raw)
}

// Get decodes the current value into dst, which must be a non-nil pointer, exactly
// as encoding/json would. JSON numbers reaching an any destination are kept as
// json.Number.
//
// The initial and an explicitly stored null both decode as JSON null, which leaves
// a non-nullable destination unchanged; decode into a pointer, an any, or a
// type with a custom unmarshaller to tell null apart from a stored zero.
//
// Get reads local state only: it produces no operation and changes neither the
// datatype version nor the push buffer, so it is allowed in every state.
func (v *Variable) Get(dst any) error {
	defer runtime.KeepAlive(v)
	var value C.QortooOwnedBytes
	var cerr C.QortooError
	C.qortoo_variable_get_raw(v.ptr, &value, &cerr)
	raw := takeOwnedBytes(&value)
	if err := takeError(&cerr); err != nil {
		return err
	}
	return newJSONDecoder(raw).Decode(dst)
}

// newJSONDecoder returns the decoder every value crossing this boundary is read
// with. UseNumber keeps a JSON number that lands in an any exact instead of
// rounding it through float64; it does not affect a typed destination.
func newJSONDecoder(raw []byte) *json.Decoder {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	return dec
}

// decodeJSONValue decodes SDK-produced JSON bytes into a dynamic Go value. The core
// guarantees exactly one valid JSON value, so a failure here means the boundary is
// broken rather than the input is bad.
func decodeJSONValue(raw []byte) (any, error) {
	var out any
	if err := newJSONDecoder(raw).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

// Transaction executes fn atomically: if fn returns an error (or panics), every
// operation performed through tx is rolled back — restoring both the value and the
// timestamp the variable held before the transaction. fn runs inline on the calling
// goroutine; tx is only valid during the call. Use TransactionContext to keep the
// commit inside the trace of a calling span.
func (v *Variable) Transaction(tag string, fn func(tx *Variable) error) error {
	defer runtime.KeepAlive(v)
	return runTransaction(tag, fn, func(cTag *C.char, userdata C.uintptr_t, cerr *C.QortooError) {
		C.qortoo_variable_transaction(v.ptr, cTag, C.qortooGoVariableTxCB(), userdata, cerr)
	})
}

// TransactionContext is Transaction continuing the trace of ctx: the commit and any
// sync it triggers become children of the span in ctx. Without a span in ctx it
// behaves exactly like Transaction. Cancellation of ctx is not honoured — fn runs
// inline on the calling goroutine.
func (v *Variable) TransactionContext(ctx context.Context, tag string, fn func(tx *Variable) error) error {
	defer runtime.KeepAlive(v)
	return runTransaction(tag, fn, func(cTag *C.char, userdata C.uintptr_t, cerr *C.QortooError) {
		withTraceContext(ctx, func(traceparent, tracestate *C.char) {
			C.qortoo_variable_transaction_with_context(
				v.ptr, cTag, traceparent, tracestate, C.qortooGoVariableTxCB(), userdata, cerr)
		})
	})
}

// Close releases this handle. The underlying datatype stays registered in its
// client. No-op for transaction-scoped handles. Close must not be called
// concurrently with other methods on the same object.
func (v *Variable) Close() {
	if v.release() {
		C.qortoo_variable_free(v.ptr)
	}
	v.ptr = nil
}
