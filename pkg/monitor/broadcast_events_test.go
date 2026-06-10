package monitor_test

import (
	"context"
	"errors"
	"log/slog"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/monitor"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/monitor/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// fakeStreamer emits a scripted list of events then blocks until ctx is done.
type fakeStreamer struct {
	events      []wdk.BroadcastStatusEvent
	lastEventID string // captured on first call
}

func (f *fakeStreamer) BroadcastStatusEvents(ctx context.Context, lastEventID string, onEvent func(wdk.BroadcastStatusEvent) error) error {
	f.lastEventID = lastEventID
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

	streamer := &fakeStreamer{}

	ctx, cancel := context.WithCancel(t.Context())
	daemon := newTestDaemon(t, logger, storage, streamer)
	require.NoError(t, daemon.Start(ctx, nil))

	// Give the goroutine a moment to call BroadcastStatusEvents (GetKeyValue is called first).
	waitForCondition(t, func() bool { return storage.GetKeyValueCalled.Load() >= 1 })

	cancel() // let the goroutine exit cleanly

	assert.Equal(t, "cursor-42", streamer.lastEventID)
}

func TestBroadcastEvents_EventsForwardedToStorage(t *testing.T) {
	t.Parallel()

	logger := logging.NewTestLogger(t)
	storage := &testabilities.MockStorage{}
	streamer := &fakeStreamer{
		events: []wdk.BroadcastStatusEvent{
			{EventID: "1", TxID: "aa"},
			{EventID: "2", TxID: "bb"},
			{EventID: "3", TxID: "cc"},
		},
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	daemon := newTestDaemon(t, logger, storage, streamer)
	require.NoError(t, daemon.Start(ctx, nil))

	// Wait until all three events have been processed.
	waitForCondition(t, func() bool { return storage.ProcessExternalTxStatusUpdateCalled.Load() >= 3 })

	events := storage.ExternalEvents()
	require.Len(t, events, 3)
	assert.Equal(t, "aa", events[0].TxID)
	assert.Equal(t, "bb", events[1].TxID)
	assert.Equal(t, "cc", events[2].TxID)
}

func TestBroadcastEvents_EventIDPersistedAfterEachEvent(t *testing.T) {
	t.Parallel()

	logger := logging.NewTestLogger(t)
	storage := &testabilities.MockStorage{}
	streamer := &fakeStreamer{
		events: []wdk.BroadcastStatusEvent{
			{EventID: "evt-1", TxID: "aa"},
			{EventID: "evt-2", TxID: "bb"},
		},
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	daemon := newTestDaemon(t, logger, storage, streamer)
	require.NoError(t, daemon.Start(ctx, nil))

	// Wait until both events have been processed.
	waitForCondition(t, func() bool { return storage.ProcessExternalTxStatusUpdateCalled.Load() >= 2 })

	val, found, err := storage.GetKeyValue(t.Context(), monitor.LastEventIDKey)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "evt-2", string(val))
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

	streamer := &fakeStreamer{
		events: []wdk.BroadcastStatusEvent{
			{EventID: "e1", TxID: "proven-tx"},
		},
	}

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
	waitForCondition(t, func() bool {
		select {
		case msg = <-provenCh:
			return true
		default:
			return false
		}
	})

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

	streamer := &fakeStreamer{
		events: []wdk.BroadcastStatusEvent{
			{EventID: "e1", TxID: "bad-tx"},  // will error
			{EventID: "e2", TxID: "good-tx"}, // must still be processed
		},
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	daemon := newTestDaemon(t, logger, storage, streamer)
	require.NoError(t, daemon.Start(ctx, nil))

	// Wait until the good-tx is processed by the inner mock.
	waitForCondition(t, func() bool {
		events := inner.ExternalEvents()
		for _, e := range events {
			if e.TxID == "good-tx" {
				return true
			}
		}
		return false
	})
}

func TestBroadcastEvents_EventIDPersistedOnStorageError(t *testing.T) {
	t.Parallel()

	logger := logging.NewTestLogger(t)
	inner := &testabilities.MockStorage{}
	storage := &fakeErrorStorage{
		MockStorage: inner,
		errOnTxID:   "bad-tx",
	}

	streamer := &fakeStreamer{
		events: []wdk.BroadcastStatusEvent{
			{EventID: "cursor-bad", TxID: "bad-tx"},
		},
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	daemon := newTestDaemon(t, logger, storage, streamer)
	require.NoError(t, daemon.Start(ctx, nil))

	// Wait until the key-value is persisted (SetKeyValue is on the inner mock).
	waitForCondition(t, func() bool { return inner.SetKeyValueCalled.Load() >= 1 })

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

// waitForCondition polls cond in a tight loop until it returns true or the test
// context deadline is reached.
func waitForCondition(t *testing.T, cond func() bool) {
	t.Helper()
	ctx := t.Context()
	for {
		select {
		case <-ctx.Done():
			t.Fatal("timed out waiting for condition")
		default:
			if cond() {
				return
			}
			// yield to the scheduler
			runtime.Gosched()
		}
	}
}
