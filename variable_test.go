package qortoo

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

type profile struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

// node builds a cyclic value, which encoding/json cannot marshal.
type node struct {
	Next *node `json:"next"`
}

// newVariable builds a writable variable for a test and closes it afterwards.
func newVariable(t *testing.T, alias, key string) *Variable {
	t.Helper()
	client, err := NewClient("go-binding-test", alias)
	require.NoError(t, err)
	t.Cleanup(client.Close)

	variable, err := client.CreateVariable(key, nil)
	require.NoError(t, err)
	t.Cleanup(variable.Close)
	return variable
}

func TestVariableStartsAtJSONNull(t *testing.T) {
	variable := newVariable(t, "variable-initial", "initial-variable")

	require.Equal(t, "initial-variable", variable.Key())
	require.Equal(t, TypeVariable, variable.Type())

	var value any
	require.NoError(t, variable.Get(&value))
	require.Nil(t, value, "a variable holds JSON null until the first Set")
	require.Zero(t, variable.ClientVersion(), "Get must not produce an operation")
}

func TestVariableRoundTripsJSONShapes(t *testing.T) {
	tests := []struct {
		name  string
		value any
		check func(t *testing.T, variable *Variable)
	}{
		{"string", "hello", func(t *testing.T, variable *Variable) {
			var got string
			require.NoError(t, variable.Get(&got))
			require.Equal(t, "hello", got)
		}},
		{"bool", true, func(t *testing.T, variable *Variable) {
			var got bool
			require.NoError(t, variable.Get(&got))
			require.True(t, got)
		}},
		{"number", 42, func(t *testing.T, variable *Variable) {
			var got int
			require.NoError(t, variable.Get(&got))
			require.Equal(t, 42, got)
		}},
		{"struct", profile{Name: "ada", Age: 36}, func(t *testing.T, variable *Variable) {
			var got profile
			require.NoError(t, variable.Get(&got))
			require.Equal(t, profile{Name: "ada", Age: 36}, got)
		}},
		{"slice", []string{"a", "b"}, func(t *testing.T, variable *Variable) {
			var got []string
			require.NoError(t, variable.Get(&got))
			require.Equal(t, []string{"a", "b"}, got)
		}},
		{"map", map[string]int{"x": 1, "y": 2}, func(t *testing.T, variable *Variable) {
			var got map[string]int
			require.NoError(t, variable.Get(&got))
			require.Equal(t, map[string]int{"x": 1, "y": 2}, got)
		}},
		{"nil", nil, func(t *testing.T, variable *Variable) {
			var got *profile
			require.NoError(t, variable.Get(&got))
			require.Nil(t, got, "a nullable destination tells JSON null apart from a value")

			var zero profile
			require.NoError(t, variable.Get(&zero))
			require.Equal(t, profile{}, zero, "JSON null leaves a non-nullable destination at its zero value")
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			variable := newVariable(t, "variable-shapes-"+tt.name, "shape-variable")
			_, err := variable.Set(tt.value)
			require.NoError(t, err)
			tt.check(t, variable)
		})
	}
}

func TestVariableSetReturnsThePreviousValue(t *testing.T) {
	variable := newVariable(t, "variable-previous", "previous-variable")

	previous, err := variable.Set("first")
	require.NoError(t, err)
	require.Nil(t, previous, "the first Set reports the initial JSON null as nil")

	// A variable may hold a different shape on every Set, so the previous value
	// is dynamic and independent of the type being written.
	previous, err = variable.Set(profile{Name: "ada", Age: 36})
	require.NoError(t, err)
	require.Equal(t, "first", previous)

	previous, err = variable.Set(7)
	require.NoError(t, err)
	require.Equal(t, map[string]any{"name": "ada", "age": json.Number("36")}, previous)

	previous, err = variable.Set(nil)
	require.NoError(t, err)
	require.Equal(t, json.Number("7"), previous, "numbers keep their exact text as json.Number")

	// An explicitly stored null is reported exactly like the initial null.
	previous, err = variable.Set("after-null")
	require.NoError(t, err)
	require.Nil(t, previous)
}

func TestVariableKeepsNumbersExactForDynamicDestinations(t *testing.T) {
	variable := newVariable(t, "variable-numbers", "number-variable")

	// A float64 round trip would lose the last digits of this integer.
	const big = "12345678901234567890"
	_, err := variable.Set(json.RawMessage(big))
	require.NoError(t, err)

	var value any
	require.NoError(t, variable.Get(&value))
	require.Equal(t, json.Number(big), value)

	// Nested numbers reaching an any are kept exact too.
	_, err = variable.Set(map[string]json.RawMessage{"n": json.RawMessage(big)})
	require.NoError(t, err)
	var nested map[string]any
	require.NoError(t, variable.Get(&nested))
	require.Equal(t, map[string]any{"n": json.Number(big)}, nested)
}

func TestVariableStoresJSONBytesVerbatim(t *testing.T) {
	variable := newVariable(t, "variable-verbatim", "verbatim-variable")

	// The SDK never reparses or reorders the stored bytes, so a reader in any
	// language sees the properties in the order encoding/json produced them.
	raw := json.RawMessage(`{"b":1,"a":2}`)
	_, err := variable.Set(raw)
	require.NoError(t, err)

	var got json.RawMessage
	require.NoError(t, variable.Get(&got))
	require.Equal(t, string(raw), string(got))
}

func TestVariableSetRejectsUnmarshallableValues(t *testing.T) {
	variable := newVariable(t, "variable-marshal", "marshal-variable")

	_, err := variable.Set("kept")
	require.NoError(t, err)
	version := variable.ClientVersion()

	_, err = variable.Set(make(chan int))
	var unsupported *json.UnsupportedTypeError
	require.ErrorAs(t, err, &unsupported, "the encoding/json error must be returned as-is")

	cyclic := &node{}
	cyclic.Next = cyclic
	_, err = variable.Set(cyclic)
	var unsupportedValue *json.UnsupportedValueError
	require.ErrorAs(t, err, &unsupportedValue, "a cycle cannot be encoded as JSON")

	require.Equal(t, version, variable.ClientVersion(), "a marshalling failure must not reach the native side")
	var value string
	require.NoError(t, variable.Get(&value))
	require.Equal(t, "kept", value)
}

func TestVariableGetRejectsInvalidDestinations(t *testing.T) {
	variable := newVariable(t, "variable-destination", "destination-variable")
	_, err := variable.Set(profile{Name: "ada", Age: 36})
	require.NoError(t, err)

	var mismatched int
	require.Error(t, variable.Get(&mismatched), "an object cannot decode into an int")

	var nonPointer profile
	require.Error(t, variable.Get(nonPointer), "a non-pointer destination cannot be written to")
	require.Error(t, variable.Get(nil))

	// None of that touched the stored value.
	var value profile
	require.NoError(t, variable.Get(&value))
	require.Equal(t, profile{Name: "ada", Age: 36}, value)
}

func TestVariableTransactionCommitsAndRestoresExactly(t *testing.T) {
	variable := newVariable(t, "variable-transaction", "transaction-variable")

	_, err := variable.Set("before")
	require.NoError(t, err)

	require.NoError(t, variable.Transaction("commit", func(tx *Variable) error {
		if _, err := tx.Set("inside"); err != nil {
			return err
		}
		_, err := tx.Set("committed")
		return err
	}))
	var value string
	require.NoError(t, variable.Get(&value))
	require.Equal(t, "committed", value)

	// Every Set of an aborted transaction is undone, not just the last one.
	wantErr := fmt.Errorf("intentional failure")
	err = variable.Transaction("rollback", func(tx *Variable) error {
		if _, err := tx.Set("one"); err != nil {
			return err
		}
		if _, err := tx.Set("two"); err != nil {
			return err
		}
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)
	require.NoError(t, variable.Get(&value))
	require.Equal(t, "committed", value)

	err = variable.Transaction("panic", func(tx *Variable) error {
		if _, err := tx.Set("panicking"); err != nil {
			return err
		}
		panic("boom")
	})
	require.Error(t, err)
	require.NoError(t, variable.Get(&value))
	require.Equal(t, "committed", value)
}

func TestVariableTransactionContextRollsBack(t *testing.T) {
	variable := newVariable(t, "variable-tx-context", "tx-context-variable")

	require.NoError(t, variable.TransactionContext(contextWithSpan(t, ""), "commit", func(tx *Variable) error {
		_, err := tx.Set("committed")
		return err
	}))

	wantErr := context.DeadlineExceeded
	err := variable.TransactionContext(contextWithSpan(t, ""), "rollback", func(tx *Variable) error {
		if _, err := tx.Set("aborted"); err != nil {
			return err
		}
		return wantErr
	})
	require.ErrorIs(t, err, wantErr, "the body's error must survive the FFI round trip")

	var value string
	require.NoError(t, variable.Get(&value))
	require.Equal(t, "committed", value)
}

func TestVariableSyncBetweenTwoClients(t *testing.T) {
	conn := NewLocalConnectivity()
	defer conn.Close()
	conn.SetRealtime(false)

	client1, err := NewClient("go-binding-test", "variable-sync-a", WithLocalConnectivity(conn))
	require.NoError(t, err)
	defer client1.Close()
	client2, err := NewClient("go-binding-test", "variable-sync-b", WithLocalConnectivity(conn))
	require.NoError(t, err)
	defer client2.Close()

	variable1, err := client1.SubscribeOrCreateVariable("shared-variable", nil)
	require.NoError(t, err)
	defer variable1.Close()

	require.Equal(t, StateSubscribingOrCreating, variable1.State())
	require.Zero(t, variable1.ServerVersion())
	require.Zero(t, variable1.SyncedClientVersion())

	_, err = variable1.Set(profile{Name: "ada", Age: 36})
	require.NoError(t, err)
	require.Equal(t, uint64(1), variable1.ClientVersion())
	require.NoError(t, variable1.SyncContext(contextWithSpan(t, "")))
	require.Equal(t, StateSubscribed, variable1.State())
	require.Equal(t, variable1.ClientVersion(), variable1.SyncedClientVersion())

	variable2, err := client2.SubscribeVariable("shared-variable", nil)
	require.NoError(t, err)
	defer variable2.Close()
	require.Equal(t, StateSubscribing, variable2.State())

	require.NoError(t, variable2.Sync())
	var got profile
	require.NoError(t, variable2.Get(&got))
	require.Equal(t, profile{Name: "ada", Age: 36}, got)

	// The later write wins on both replicas.
	_, err = variable2.Set("overwritten")
	require.NoError(t, err)
	require.NoError(t, variable2.Sync())
	require.NoError(t, variable1.Sync())

	var value string
	require.NoError(t, variable1.Get(&value))
	require.Equal(t, "overwritten", value)

	require.NoError(t, variable2.Unsubscribe())
	require.Equal(t, StateUnsubscribing, variable2.State())
}

func TestVariableSetAndUnsetHandler(t *testing.T) {
	variable := newVariable(t, "variable-handler", "handler-variable")

	variable.SetHandler(1, &Handler{OnStateChange: func(oldState, newState DatatypeState) {}})
	require.True(t, variable.UnsetHandler(1))
	require.False(t, variable.UnsetHandler(1))
}
