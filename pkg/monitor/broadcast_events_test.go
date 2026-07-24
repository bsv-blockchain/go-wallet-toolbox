package monitor_test

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/monitor"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/monitor/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// fakeStreamer emits a scripted list of events then blocks until ctx is done.
type fakeStreamer struct {
	events []wdk.BroadcastStatusEvent

	mu          sync.Mutex
	lastEventID string // captured on first call; protected by mu

	// started is closed once BroadcastStatusEvents has been entered and
	// lastEventID has been captured.  Tests can wait on this channel to ensure
	// the goroutine has progressed past the cursor read before asserting.
	started chan struct{}
}

func newFakeStreamer(events []wdk.BroadcastStatusEvent) *fakeStreamer {
	return &fakeStreamer{events: events, started: make(chan struct{})}
}

func (f *fakeStreamer) LastEventID() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lastEventID
}

func (f *fakeStreamer) BroadcastStatusEvents(ctx context.Context, lastEventID string, onEvent func(wdk.BroadcastStatusEvent) error) error {
	f.mu.Lock()
	f.lastEventID = lastEventID
	f.mu.Unlock()

	// Signal once that BroadcastStatusEvents has been entered and lastEventID set.
	select {
	case <-f.started:
		// already closed (reconnect attempt) — do nothing
	default:
		close(f.started)
	}

	for _, ev := range f.events {
		if err := onEvent(ev); err != nil {
			return err
		}
	}
	// Block until ctx done (simulates the real SSE stream staying connected).
	<-ctx.Done()
	return nil
}

// fakeErrorStorage wraps MockStorage and injects errors for specific events.
type fakeErrorStorage struct {
	*testabilities.MockStorage

	errOnTxID string
}

func (f *fakeErrorStorage) ProcessExternalTxStatusUpdate(ctx context.Context, ev wdk.BroadcastStatusEvent) ([]wdk.TxSynchronizedStatus, error) {
	if ev.TxID == f.errOnTxID {
		return nil, errors.New("injected storage error")
	}
	return f.MockStorage.ProcessExternalTxStatusUpdate(ctx, ev)
}

// newTestDaemon builds a Daemon wired to the supplied storage and streamer.
func newTestDaemon(t *testing.T, logger *slog.Logger, storage monitor.MonitoredStorage, streamer monitor.BroadcastEventStreamer) *monitor.Daemon {
	t.Helper()

	var opts []monitor.DaemonEventOption
	if streamer != nil {
		opts = append(opts, monitor.WithBroadcastEventStream(streamer))
	}

	daemon, err := monitor.NewDaemon(logger, storage, monitor.DefaultDaemonEventOptions(opts...))
	require.NoError(t, err)

	t.Cleanup(func() { _ = daemon.Stop() })
	return daemon
}

func TestBroadcastEvents_LastEventIDPassedToStreamer(t *testing.T) {
	t.Parallel()

	logger := logging.NewTestLogger(t)
	storage := &testabilities.MockStorage{}
	// Pre-seed a last-event-id in storage.
	require.NoError(t, storage.SetKeyValue(t.Context(), monitor.LastEventIDKey, []byte("cursor-42")))

	streamer := newFakeStreamer(nil)

	ctx, cancel := context.WithCancel(t.Context())
	daemon := newTestDaemon(t, logger, storage, streamer)
	require.NoError(t, daemon.Start(ctx, nil))

	// Wait until BroadcastStatusEvents has been entered and lastEventID captured
	// before asserting — this prevents a data race between the goroutine write
	// and the test read.
	select {
	case <-streamer.started:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for BroadcastStatusEvents to be entered")
	}

	cancel() // let the goroutine exit cleanly

	assert.Equal(t, "cursor-42", streamer.LastEventID())
}

func TestBroadcastEvents_EventsForwardedToStorage(t *testing.T) {
	t.Parallel()

	logger := logging.NewTestLogger(t)
	storage := &testabilities.MockStorage{}
	streamer := newFakeStreamer([]wdk.BroadcastStatusEvent{
		{EventID: "1", TxID: "aa"},
		{EventID: "2", TxID: "bb"},
		{EventID: "3", TxID: "cc"},
	})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	daemon := newTestDaemon(t, logger, storage, streamer)
	require.NoError(t, daemon.Start(ctx, nil))

	// Wait until all three events have been processed.
	require.Eventually(t, func() bool { return storage.ProcessExternalTxStatusUpdateCalled.Load() >= 3 }, 5*time.Second, time.Millisecond)

	// Events apply through a bounded worker pool, so arrival order at storage
	// is not guaranteed — assert on the set.
	events := storage.ExternalEvents()
	require.Len(t, events, 3)
	got := make([]string, 0, len(events))
	for _, ev := range events {
		got = append(got, ev.TxID)
	}
	assert.ElementsMatch(t, []string{"aa", "bb", "cc"}, got)
}

func TestBroadcastEvents_EventIDPersistedPerBatch(t *testing.T) {
	t.Parallel()

	logger := logging.NewTestLogger(t)
	storage := &testabilities.MockStorage{}
	streamer := newFakeStreamer([]wdk.BroadcastStatusEvent{
		{EventID: "evt-1", TxID: "aa"},
		{EventID: "evt-2", TxID: "bb"},
	})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	daemon := newTestDaemon(t, logger, storage, streamer)
	require.NoError(t, daemon.Start(ctx, nil))

	// Events apply in batches with one cursor write per batch; regardless of
	// batching, the durable cursor must end at the newest event ID.
	require.Eventually(t, func() bool {
		val, found, err := storage.GetKeyValue(t.Context(), monitor.LastEventIDKey)
		return err == nil && found && string(val) == "evt-2"
	}, 5*time.Second, time.Millisecond)
}

func TestBroadcastEvents_ProvenEventsForwarded(t *testing.T) {
	t.Parallel()

	logger := logging.NewTestLogger(t)

	provenCh := make(chan wdk.CurrentTxStatus, 10)

	// Use a storage that returns a proven result for one event.
	storage := &provenMockStorage{
		MockStorage:  &testabilities.MockStorage{},
		provenTxID:   "proven-tx",
		provenResult: wdk.TxSynchronizedStatus{TxID: "proven-tx", Status: wdk.ProvenTxStatusCompleted},
	}

	streamer := newFakeStreamer([]wdk.BroadcastStatusEvent{
		{EventID: "e1", TxID: "proven-tx"},
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

	// Wait until the proven event lands on the channel.
	var msg wdk.CurrentTxStatus
	require.Eventually(t, func() bool {
		select {
		case msg = <-provenCh:
			return true
		default:
			return false
		}
	}, 5*time.Second, time.Millisecond)

	assert.Equal(t, "proven-tx", msg.TxID)
}

func TestBroadcastEvents_StorageErrorDoesNotStopStream(t *testing.T) {
	t.Parallel()

	logger := logging.NewTestLogger(t)
	inner := &testabilities.MockStorage{}
	storage := &fakeErrorStorage{
		MockStorage: inner,
		errOnTxID:   "bad-tx", // storage.ProcessExternalTxStatusUpdate will fail for this tx
	}

	streamer := newFakeStreamer([]wdk.BroadcastStatusEvent{
		{EventID: "e1", TxID: "bad-tx"},  // will error
		{EventID: "e2", TxID: "good-tx"}, // must still be processed
	})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	daemon := newTestDaemon(t, logger, storage, streamer)
	require.NoError(t, daemon.Start(ctx, nil))

	// Wait until the good-tx is processed by the inner mock.
	require.Eventually(t, func() bool {
		events := inner.ExternalEvents()
		for _, e := range events {
			if e.TxID == "good-tx" {
				return true
			}
		}
		return false
	}, 5*time.Second, time.Millisecond)
}

func TestBroadcastEvents_EventIDPersistedOnStorageError(t *testing.T) {
	t.Parallel()

	logger := logging.NewTestLogger(t)
	inner := &testabilities.MockStorage{}
	storage := &fakeErrorStorage{
		MockStorage: inner,
		errOnTxID:   "bad-tx",
	}

	streamer := newFakeStreamer([]wdk.BroadcastStatusEvent{
		{EventID: "cursor-bad", TxID: "bad-tx"},
	})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	daemon := newTestDaemon(t, logger, storage, streamer)
	require.NoError(t, daemon.Start(ctx, nil))

	// Wait until the key-value is persisted (SetKeyValue is on the inner mock).
	require.Eventually(t, func() bool { return inner.SetKeyValueCalled.Load() >= 1 }, 5*time.Second, time.Millisecond)

	val, found, err := inner.GetKeyValue(t.Context(), monitor.LastEventIDKey)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "cursor-bad", string(val))
}

// provenMockStorage wraps MockStorage and returns a proven result for a specific tx.
type provenMockStorage struct {
	*testabilities.MockStorage

	provenTxID   string
	provenResult wdk.TxSynchronizedStatus
}

func (p *provenMockStorage) ProcessExternalTxStatusUpdate(ctx context.Context, ev wdk.BroadcastStatusEvent) ([]wdk.TxSynchronizedStatus, error) {
	p.ProcessExternalTxStatusUpdateCalled.Add(1)
	if ev.TxID == p.provenTxID {
		return []wdk.TxSynchronizedStatus{p.provenResult}, nil
	}
	return nil, nil
}
