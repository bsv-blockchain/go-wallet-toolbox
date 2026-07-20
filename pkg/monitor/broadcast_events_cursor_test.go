package monitor_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/monitor"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/monitor/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// reconnectingStreamer emits its scripted events on the first call, then returns
// nil so the daemon's reconnect loop invokes it again. It records the lastEventID
// passed to every call and blocks on the reconnect call until ctx is done.
type reconnectingStreamer struct {
	events []wdk.BroadcastStatusEvent

	mu      sync.Mutex
	cursors []string // lastEventID received on each call, in order; protected by mu

	// reconnected is closed once BroadcastStatusEvents has been entered for the
	// second time, i.e. the cursor used for the reconnect attempt has been
	// captured.  Tests wait on this channel before asserting on cursors.
	reconnected chan struct{}
}

func newReconnectingStreamer(events []wdk.BroadcastStatusEvent) *reconnectingStreamer {
	return &reconnectingStreamer{events: events, reconnected: make(chan struct{})}
}

func (r *reconnectingStreamer) Cursors() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	cursors := make([]string, len(r.cursors))
	copy(cursors, r.cursors)
	return cursors
}

func (r *reconnectingStreamer) BroadcastStatusEvents(ctx context.Context, lastEventID string, onEvent func(wdk.BroadcastStatusEvent) error) error {
	r.mu.Lock()
	r.cursors = append(r.cursors, lastEventID)
	call := len(r.cursors)
	r.mu.Unlock()

	if call == 1 {
		for _, ev := range r.events {
			if err := onEvent(ev); err != nil {
				return err
			}
		}
		// Return without error so the reconnect loop calls us again with the
		// cursor it believes is current.
		return nil
	}

	// Reconnect attempt: signal that the reconnect cursor has been captured.
	select {
	case <-r.reconnected:
		// already closed (later reconnect attempt) — do nothing
	default:
		close(r.reconnected)
	}
	// Block until ctx done (simulates the real SSE stream staying connected).
	<-ctx.Done()
	return nil
}

// waitForReconnect blocks until the streamer has been entered a second time.
func waitForReconnect(t *testing.T, streamer *reconnectingStreamer) {
	t.Helper()

	select {
	case <-streamer.reconnected:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the streamer to be reconnected")
	}
}

// failingPersistStorage wraps provenMockStorage and fails every SetKeyValue call
// without storing the value, simulating a persist failure of the replay cursor.
type failingPersistStorage struct {
	*provenMockStorage

	SetKeyValueAttempts atomic.Int64
}

func (f *failingPersistStorage) SetKeyValue(_ context.Context, _ string, _ []byte) error {
	f.SetKeyValueAttempts.Add(1)
	return errors.New("injected persist error")
}

func TestBroadcastEvents_EmptyEventIDDoesNotAdvanceCursor(t *testing.T) {
	t.Parallel()

	logger := logging.NewTestLogger(t)
	storage := &testabilities.MockStorage{}
	// Pre-seed a last-event-id in storage.
	require.NoError(t, storage.SetKeyValue(t.Context(), monitor.LastEventIDKey, []byte("cursor-orig")))
	setKeyValueCallsAfterSeed := storage.SetKeyValueCalled.Load()

	streamer := newReconnectingStreamer([]wdk.BroadcastStatusEvent{
		{EventID: "", TxID: "no-id-tx"},
	})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	daemon := newTestDaemon(t, logger, storage, streamer)
	require.NoError(t, daemon.Start(ctx, nil))

	waitForReconnect(t, streamer)

	// The event itself must still have been processed.
	events := storage.ExternalEvents()
	require.Len(t, events, 1)
	assert.Equal(t, "no-id-tx", events[0].TxID)

	// SetKeyValue must not have been called for the empty-ID event.
	assert.Equal(t, setKeyValueCallsAfterSeed, storage.SetKeyValueCalled.Load())

	// The reconnect attempt must resume from the original cursor.
	cursors := streamer.Cursors()
	require.Len(t, cursors, 2)
	assert.Equal(t, "cursor-orig", cursors[0])
	assert.Equal(t, "cursor-orig", cursors[1])

	// The persisted cursor must be untouched.
	val, found, err := storage.GetKeyValue(t.Context(), monitor.LastEventIDKey)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "cursor-orig", string(val))
}

func TestBroadcastEvents_PersistErrorDoesNotAdvanceCursor(t *testing.T) {
	t.Parallel()

	logger := logging.NewTestLogger(t)

	provenCh := make(chan wdk.CurrentTxStatus, 10)

	inner := &testabilities.MockStorage{}
	// Pre-seed a last-event-id in the inner storage (the wrapper only fails writes).
	require.NoError(t, inner.SetKeyValue(t.Context(), monitor.LastEventIDKey, []byte("cursor-orig")))

	storage := &failingPersistStorage{
		provenMockStorage: &provenMockStorage{
			MockStorage:  inner,
			provenTxID:   "proven-tx",
			provenResult: wdk.TxSynchronizedStatus{TxID: "proven-tx", Status: wdk.ProvenTxStatusCompleted},
		},
	}

	streamer := newReconnectingStreamer([]wdk.BroadcastStatusEvent{
		{EventID: "evt-new", TxID: "proven-tx"},
	})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	opts := []monitor.DaemonEventOption{
		monitor.WithBroadcastEventStream(streamer),
		monitor.WithProvenTxChannel(provenCh),
	}
	daemon, err := monitor.NewDaemon(logger, storage, monitor.DefaultDaemonEventOptions(opts...))
	require.NoError(t, err)
	t.Cleanup(func() { _ = daemon.Stop() })
	require.NoError(t, daemon.Start(ctx, nil))

	waitForReconnect(t, streamer)

	// The persist was attempted (and failed), and the event was still processed.
	assert.GreaterOrEqual(t, storage.SetKeyValueAttempts.Load(), int64(1))
	assert.GreaterOrEqual(t, storage.ProcessExternalTxStatusUpdateCalled.Load(), int64(1))

	// Proven events must still be forwarded despite the persist failure.
	var msg wdk.CurrentTxStatus
	select {
	case msg = <-provenCh:
	default:
		t.Fatal("expected a proven event on the channel")
	}
	assert.Equal(t, "proven-tx", msg.TxID)

	// The reconnect attempt must resume from the last durably persisted cursor.
	cursors := streamer.Cursors()
	require.Len(t, cursors, 2)
	assert.Equal(t, "cursor-orig", cursors[0])
	assert.Equal(t, "cursor-orig", cursors[1])

	// The durable cursor must be untouched.
	val, found, err := inner.GetKeyValue(t.Context(), monitor.LastEventIDKey)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "cursor-orig", string(val))
}

func TestBroadcastEvents_PersistedCursorUsedOnReconnect(t *testing.T) {
	t.Parallel()

	logger := logging.NewTestLogger(t)
	storage := &testabilities.MockStorage{}
	// Pre-seed a last-event-id in storage.
	require.NoError(t, storage.SetKeyValue(t.Context(), monitor.LastEventIDKey, []byte("cursor-orig")))

	streamer := newReconnectingStreamer([]wdk.BroadcastStatusEvent{
		{EventID: "evt-new", TxID: "aa"},
	})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	daemon := newTestDaemon(t, logger, storage, streamer)
	require.NoError(t, daemon.Start(ctx, nil))

	waitForReconnect(t, streamer)

	// The reconnect attempt must resume from the newly persisted cursor.
	cursors := streamer.Cursors()
	require.Len(t, cursors, 2)
	assert.Equal(t, "cursor-orig", cursors[0])
	assert.Equal(t, "evt-new", cursors[1])

	// The new cursor must have been persisted.
	val, found, err := storage.GetKeyValue(t.Context(), monitor.LastEventIDKey)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "evt-new", string(val))
}
