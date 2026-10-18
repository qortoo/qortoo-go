package qortoo

// Go-only diagnostics executed by the SDK's CodSpeed workflow.
// Central Rust/Go latency workloads, contracts, and official results belong to
// https://github.com/qortoo/qortoo-bench. These adaptive testing.B measurements
// are not the central fixed-budget comparison and do not isolate cgo overhead.
//
// Measurements use a release native SDK, manual LocalConnectivity, one caller
// thread, and no registered handler. Mutable scenarios stop the timer to replace
// datatypes so pending operations and local server history stay bounded.
// Go allocation counters describe the Go heap, not Rust native allocations.

import (
	"fmt"
	"testing"
)

const (
	benchCollection = "bench-collection"

	// implLabel identifies the Go API path in diagnostic output.
	implLabel = "impl=go"

	// Operations recorded per transaction in the transaction scenario.
	txOps = 10

	// Single writes performed on one datatype before it is replaced.
	resetEveryOps = 50_000

	// Transactions performed on one datatype before it is replaced.
	resetEveryTxs = resetEveryOps / txOps

	// Push-pulls performed on one datatype before it is replaced. The server
	// keeps every pushed transaction in its history, which is what this bounds.
	resetEverySyncs = 5_000

	// Counters built per client in the build diagnostic. Client recycling
	// bounds datatype registrations during adaptive testing.B runs.
	countersPerClient = 100
)

// benchSink keeps the compiler from eliding the result of read benchmarks.
var benchSink int64

// nextBenchKey is the source of unique datatype keys: a key can only be
// registered once per client.
var nextBenchKey int

// benchClient owns a client and the connectivity it was built with, so a
// benchmark that recycles clients can release both — each client runs its own
// tokio runtime, and leaving them open exhausts the OS thread limit.
type benchClient struct {
	client *Client
	conn   *LocalConnectivity
}

func (bc *benchClient) close() {
	bc.client.Close()
	bc.conn.Close()
}

// newBenchClient builds a client whose connectivity never syncs on its own.
func newBenchClient(b *testing.B, alias string) *benchClient {
	b.Helper()
	conn := NewLocalConnectivity()
	conn.SetRealtime(false)
	client, err := NewClient(benchCollection, alias, WithLocalConnectivity(conn))
	if err != nil {
		b.Fatalf("client must build: %v", err)
	}
	return &benchClient{client: client, conn: conn}
}

// newManagedBenchClient is newBenchClient for benchmarks that keep one client
// for their whole run.
func newManagedBenchClient(b *testing.B, alias string) *Client {
	b.Helper()
	bc := newBenchClient(b, alias)
	b.Cleanup(bc.close)
	return bc.client
}

func nextBenchCounter(b *testing.B, client *Client) *Counter {
	b.Helper()
	nextBenchKey++
	counter, err := client.CreateCounter(fmt.Sprintf("bench-%d", nextBenchKey), nil)
	if err != nil {
		b.Fatalf("counter must build: %v", err)
	}
	return counter
}

// benchWithReset runs op b.N times, replacing the datatype every resetEvery
// operations with the timer stopped.
func benchWithReset(b *testing.B, alias string, resetEvery int, op func(*Counter)) {
	b.Helper()
	client := newManagedBenchClient(b, alias)

	var counter *Counter
	b.Cleanup(func() {
		if counter != nil {
			counter.Close()
		}
	})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if i%resetEvery == 0 {
			b.StopTimer()
			if counter != nil {
				counter.Close()
			}
			counter = nextBenchCounter(b, client)
			b.StartTimer()
		}
		op(counter)
	}
}

// BenchmarkIncreaseBy measures the cheapest mutating operation.
func BenchmarkIncreaseBy(b *testing.B) {
	b.Run(implLabel, func(b *testing.B) {
		benchWithReset(b, "bench-increase-by", resetEveryOps, func(counter *Counter) {
			if _, err := counter.IncreaseBy(1); err != nil {
				b.Fatalf("increase must succeed: %v", err)
			}
		})
	})
}

// BenchmarkGetValue measures the cheapest call of the whole API, where the
// call overhead is visible in the complete Go API path.
func BenchmarkGetValue(b *testing.B) {
	b.Run(implLabel, func(b *testing.B) {
		client := newManagedBenchClient(b, "bench-get-value")
		counter := nextBenchCounter(b, client)
		b.Cleanup(counter.Close)
		if _, err := counter.Increase(); err != nil {
			b.Fatalf("increase must succeed: %v", err)
		}

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			benchSink = counter.Value()
		}
	})
}

// BenchmarkTransaction measures a transaction of txOps operations, which crosses
// the callback trampoline (Go → Rust → Go).
func BenchmarkTransaction(b *testing.B) {
	b.Run(implLabel, func(b *testing.B) {
		benchWithReset(b, "bench-transaction", resetEveryTxs, func(counter *Counter) {
			err := counter.Transaction("bench", func(tx *Counter) error {
				for range txOps {
					if _, err := tx.IncreaseBy(1); err != nil {
						return err
					}
				}
				return nil
			})
			if err != nil {
				b.Fatalf("transaction must commit: %v", err)
			}
		})
	})
}

// BenchmarkSyncLocal measures one write plus a manual push-pull against the
// in-memory server: the protocol path, where the binding should be negligible.
func BenchmarkSyncLocal(b *testing.B) {
	b.Run(implLabel, func(b *testing.B) {
		benchWithReset(b, "bench-sync-local", resetEverySyncs, func(counter *Counter) {
			if _, err := counter.IncreaseBy(1); err != nil {
				b.Fatalf("increase must succeed: %v", err)
			}
			if err := counter.Sync(); err != nil {
				b.Fatalf("sync must succeed: %v", err)
			}
		})
	})
}

// BenchmarkBuildCounter measures building and closing a counter — the one-time
// cost per datatype, including this binding's runtime.AddCleanup registration.
// The per-build cost is not constant; the client is recycled to bound registrations.
func BenchmarkBuildCounter(b *testing.B) {
	b.Run(implLabel, func(b *testing.B) {
		// Keys are unique because a key can only be registered once per client;
		// formatting them is deliberately left out of the measured region.
		keys := make([]string, b.N)
		for i := range keys {
			keys[i] = fmt.Sprintf("build-%d", i)
		}
		client := newBenchClient(b, "bench-build-counter")
		defer func() { client.close() }()

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if i > 0 && i%countersPerClient == 0 {
				// Recycling must release the previous client: every client owns
				// a tokio runtime, and keeping them all open exhausts OS threads.
				b.StopTimer()
				client.close()
				client = newBenchClient(b, "bench-build-counter")
				b.StartTimer()
			}
			counter, err := client.client.CreateCounter(keys[i], nil)
			if err != nil {
				b.Fatalf("counter must build: %v", err)
			}
			counter.Close()
		}
	})
}
