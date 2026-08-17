package qortoo

import (
	"context"
	"fmt"
	"runtime/cgo"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDatatypeStateString(t *testing.T) {
	tests := []struct {
		state DatatypeState
		want  string
	}{
		{StateCreating, "Creating"},
		{StateSubscribing, "Subscribing"},
		{StateSubscribingOrCreating, "SubscribingOrCreating"},
		{StateSubscribed, "Subscribed"},
		{StateUnsubscribing, "Unsubscribing"},
		{StateDeleting, "Deleting"},
		{StateDisabled, "Disabled"},
		{DatatypeState(42), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			require.Equal(t, tt.want, tt.state.String())
		})
	}
}

func TestClientAndCounterLifecycle(t *testing.T) {
	client, err := NewClient("go-binding-test", "lifecycle")
	require.NoError(t, err)
	defer client.Close()

	require.Equal(t, "go-binding-test", client.Collection())
	require.Equal(t, "lifecycle", client.Alias())

	counter, err := client.CreateCounter("my-counter", nil)
	require.NoError(t, err)
	defer counter.Close()

	require.Equal(t, "my-counter", counter.Key())
	require.Equal(t, TypeCounter, counter.Type())
	require.Equal(t, StateCreating, counter.State())

	v, err := counter.Increase()
	require.NoError(t, err)
	require.Equal(t, int64(1), v)
	v, err = counter.IncreaseBy(4)
	require.NoError(t, err)
	require.Equal(t, int64(5), v)
	require.Equal(t, int64(5), counter.Value())
	require.Equal(t, uint64(2), counter.ClientVersion())
}

func TestInvalidCollectionName(t *testing.T) {
	_, err := NewClient("9starts-with-digit", "invalid")
	require.Error(t, err)
	require.ErrorIs(t, err, &Error{Code: ErrCodeInvalidCollectionName})
}

func TestReadonlyCounterRejectsWrites(t *testing.T) {
	client, err := NewClient("go-binding-test", "readonly")
	require.NoError(t, err)
	defer client.Close()

	counter, err := client.CreateCounter("ro-counter", &DatatypeOptions{Readonly: true})
	require.NoError(t, err)
	defer counter.Close()

	_, err = counter.Increase()
	require.Error(t, err)
}

func TestSyncBetweenTwoClients(t *testing.T) {
	conn := NewLocalConnectivity()
	defer conn.Close()
	conn.SetRealtime(false)

	client1, err := NewClient("go-binding-test", "sync-a", WithLocalConnectivity(conn))
	require.NoError(t, err)
	defer client1.Close()
	client2, err := NewClient("go-binding-test", "sync-b", WithLocalConnectivity(conn))
	require.NoError(t, err)
	defer client2.Close()

	counter1, err := client1.CreateCounter("shared-counter", nil)
	require.NoError(t, err)
	defer counter1.Close()

	_, err = counter1.Increase()
	require.NoError(t, err)
	require.NoError(t, counter1.Sync())
	require.Equal(t, StateSubscribed, counter1.State())

	counter2, err := client2.SubscribeCounter("shared-counter", nil)
	require.NoError(t, err)
	defer counter2.Close()

	require.NoError(t, counter2.Sync())
	require.Equal(t, int64(1), counter2.Value())
}

func TestSubscribeOrCreateTracksSynchronizationVersions(t *testing.T) {
	conn := NewLocalConnectivity()
	defer conn.Close()
	conn.SetRealtime(false)

	client, err := NewClient("go-binding-test", "subscribe-or-create", WithLocalConnectivity(conn))
	require.NoError(t, err)
	defer client.Close()

	counter, err := client.SubscribeOrCreateCounter("subscribe-or-create-counter", nil)
	require.NoError(t, err)
	defer counter.Close()

	require.Equal(t, StateSubscribingOrCreating, counter.State())
	require.Zero(t, counter.ServerVersion())
	require.Zero(t, counter.ClientVersion())
	require.Zero(t, counter.SyncedClientVersion())

	_, err = counter.Increase()
	require.NoError(t, err)
	require.Equal(t, uint64(1), counter.ClientVersion())
	require.Zero(t, counter.ServerVersion())
	require.Zero(t, counter.SyncedClientVersion())

	require.NoError(t, counter.Sync())
	require.Equal(t, StateSubscribed, counter.State())
	require.Equal(t, counter.ClientVersion(), counter.ServerVersion())
	require.Equal(t, counter.ClientVersion(), counter.SyncedClientVersion())
}

func TestCounterAndClientUnsubscribeLifecycle(t *testing.T) {
	conn := NewLocalConnectivity()
	defer conn.Close()
	conn.SetRealtime(false)

	client, err := NewClient("go-binding-test", "unsubscribe", WithLocalConnectivity(conn))
	require.NoError(t, err)
	defer client.Close()

	counterSide, err := client.CreateCounter("counter-side-unsubscribe", nil)
	require.NoError(t, err)
	defer counterSide.Close()
	require.NoError(t, counterSide.Sync())
	require.NoError(t, counterSide.Unsubscribe())
	require.Equal(t, StateUnsubscribing, counterSide.State())
	require.NoError(t, counterSide.Sync())
	require.Equal(t, StateDisabled, counterSide.State())
	require.ErrorIs(t, counterSide.Unsubscribe(), &Error{Code: ErrCodeNotWritable})

	const clientSideKey = "client-side-unsubscribe"
	clientSide, err := client.CreateCounter(clientSideKey, nil)
	require.NoError(t, err)
	defer clientSide.Close()
	require.NoError(t, clientSide.Sync())
	require.NoError(t, client.UnsubscribeDatatype(clientSideKey))
	require.Equal(t, StateUnsubscribing, clientSide.State())
	require.NoError(t, clientSide.Sync())
	require.Equal(t, StateDisabled, clientSide.State())
	require.ErrorIs(t, client.UnsubscribeDatatype(clientSideKey), &Error{Code: ErrCodeDisallowed})
}

func TestTransactionCommitAndRollback(t *testing.T) {
	client, err := NewClient("go-binding-test", "transaction")
	require.NoError(t, err)
	defer client.Close()

	counter, err := client.CreateCounter("tx-counter", nil)
	require.NoError(t, err)
	defer counter.Close()

	err = counter.Transaction("commit", func(tx *Counter) error {
		if _, err := tx.IncreaseBy(10); err != nil {
			return err
		}
		if _, err := tx.IncreaseBy(5); err != nil {
			return err
		}
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, int64(15), counter.Value())

	wantErr := fmt.Errorf("intentional failure")
	err = counter.Transaction("rollback", func(tx *Counter) error {
		if _, err := tx.IncreaseBy(100); err != nil {
			return err
		}
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)
	require.Equal(t, int64(15), counter.Value())

	err = counter.Transaction("panic", func(tx *Counter) error {
		if _, err := tx.IncreaseBy(100); err != nil {
			return err
		}
		panic("boom")
	})
	require.Error(t, err)
	require.Equal(t, int64(15), counter.Value())
}

func TestTransactionContextRollsBack(t *testing.T) {
	client, err := NewClient("go-binding-test", "context-rollback")
	require.NoError(t, err)
	defer client.Close()

	counter, err := client.CreateCounter("context-rollback-counter", nil)
	require.NoError(t, err)
	defer counter.Close()

	wantErr := context.DeadlineExceeded
	err = counter.TransactionContext(contextWithSpan(t, ""), "tx", func(tx *Counter) error {
		if _, err := tx.IncreaseBy(10); err != nil {
			return err
		}
		return wantErr
	})
	require.ErrorIs(t, err, wantErr, "the body's error must survive the FFI round trip")
	require.Equal(t, int64(0), counter.Value())
}

func TestHandlerOnStateChange(t *testing.T) {
	conn := NewLocalConnectivity()
	defer conn.Close()
	conn.SetRealtime(false)

	client, err := NewClient("go-binding-test", "handler", WithLocalConnectivity(conn))
	require.NoError(t, err)
	defer client.Close()

	type transition struct{ oldState, newState DatatypeState }
	changes := make(chan transition, 8)

	counter, err := client.CreateCounter("handler-counter", &DatatypeOptions{
		Handler: &Handler{
			OnStateChange: func(oldState, newState DatatypeState) {
				changes <- transition{oldState, newState}
			},
		},
	})
	require.NoError(t, err)
	defer counter.Close()

	_, err = counter.Increase()
	require.NoError(t, err)
	require.NoError(t, counter.Sync())

	select {
	case tr := <-changes:
		require.Equal(t, StateCreating, tr.oldState)
		require.Equal(t, StateSubscribed, tr.newState)
	case <-time.After(5 * time.Second):
		require.FailNow(t, "timed out waiting for OnStateChange")
	}
}

func TestSetAndUnsetHandler(t *testing.T) {
	client, err := NewClient("go-binding-test", "set-handler")
	require.NoError(t, err)
	defer client.Close()

	counter, err := client.CreateCounter("set-handler-counter", nil)
	require.NoError(t, err)
	defer counter.Close()

	counter.SetHandler(1, &Handler{
		OnStateChange: func(oldState, newState DatatypeState) {},
	})
	require.True(t, counter.UnsetHandler(1))
	require.False(t, counter.UnsetHandler(1))
}

func TestHandlerFromUserdataRejectsMissingAndUnexpectedValues(t *testing.T) {
	require.Nil(t, handlerFromUserdata(0))

	unexpected := cgo.NewHandle("not a handler")
	defer unexpected.Delete()
	require.Nil(t, handlerFromUserdata(uintptr(unexpected)))

	want := &Handler{}
	h := cgo.NewHandle(want)
	defer h.Delete()
	require.Same(t, want, handlerFromUserdata(uintptr(h)))
}

func TestHandlerCallbacksAllowNilAndContainPanics(t *testing.T) {
	conn := NewLocalConnectivity()
	defer conn.Close()
	conn.SetRealtime(false)

	client, err := NewClient("go-binding-test", "handler-safety", WithLocalConnectivity(conn))
	require.NoError(t, err)
	defer client.Close()

	counter, err := client.CreateCounter("handler-safety-counter", nil)
	require.NoError(t, err)
	defer counter.Close()

	counter.SetHandler(0, &Handler{})
	called := make(chan struct{}, 1)
	counter.SetHandler(1, &Handler{OnStateChange: func(oldState, newState DatatypeState) {
		called <- struct{}{}
		panic("intentional callback panic")
	}})

	_, err = counter.Increase()
	require.NoError(t, err)
	require.NoError(t, counter.Sync())
	require.Equal(t, StateSubscribed, counter.State())

	select {
	case <-called:
	case <-time.After(5 * time.Second):
		require.FailNow(t, "timed out waiting for panicking callback")
	}
}

func TestHandlerOnErrorCopiesCallbackMessage(t *testing.T) {
	conn := NewLocalConnectivity()
	defer conn.Close()
	conn.SetRealtime(false)

	client, err := NewClient("go-binding-test", "handler-error", WithLocalConnectivity(conn))
	require.NoError(t, err)
	defer client.Close()

	errors := make(chan Error, 1)
	counter, err := client.CreateCounter("handler-error-counter", &DatatypeOptions{
		MaxPushBufferSize: 1,
		Handler: &Handler{OnError: func(err *Error) {
			select {
			case errors <- *err:
			default:
			}
		}},
	})
	require.NoError(t, err)
	defer counter.Close()

	const safetyBound = 50_000
	var got Error
	for range safetyBound {
		_, err = counter.Increase()
		require.NoError(t, err)
		select {
		case got = <-errors:
			goto received
		default:
		}
	}

	select {
	case got = <-errors:
	case <-time.After(5 * time.Second):
		require.FailNow(t, "timed out waiting for OnError")
	}

received:
	require.Equal(t, ErrCodePushBufferExceededMaxMemSize, got.Code)
	require.NotEmpty(t, got.Msg, "the Rust-owned callback message must be copied into Go memory")
}
