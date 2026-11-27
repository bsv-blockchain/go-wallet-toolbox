package monitor_test

import (
	"context"
	"fmt"
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

// eventProvingStorage reports provenPerEvent proven transactions for every stream event.
type eventProvingStorage struct {
	*testabilities.MockStorage

	provenPerEvent int
}

func (s *eventProvingStorage) ProcessExternalTxStatusUpdate(_ context.Context, ev wdk.BroadcastStatusEvent) ([]wdk.TxSynchronizedStatus, error) {
	results := make([]wdk.TxSynchronizedStatus, 0, s.provenPerEvent)
	for i := range s.provenPerEvent {
		results = append(results, wdk.TxSynchronizedStatus{
			TxID:   fmt.Sprintf("tx-%s-%d", ev.TxID, i),
			Status: wdk.ProvenTxStatusCompleted,
		})
	}
	return results, nil
}

// chanStreamer forwards the events sent on its channel until the stream context ends.
type chanStreamer chan wdk.BroadcastStatusEvent

func (c chanStreamer) BroadcastStatusEvents(ctx context.Context, _ string, onEvent func(wdk.BroadcastStatusEvent) error) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case ev := <-c:
			if err := onEvent(ev); err != nil {
				return err
			}
		}
	}
}

func minedEvent(i uint32) wdk.BroadcastStatusEvent {
	return wdk.BroadcastStatusEvent{EventID: fmt.Sprint(i), TxID: fmt.Sprint(i), Status: "MINED"}
}

func TestProvenEvents_SlowSubscriberMissesNothing(t *testing.T) {
	t.Parallel()

	// given: a proven channel with room for one event and a burst of proofs per stream event
	const events, provenPerEvent = 5, 200
	provenCh := make(chan wdk.CurrentTxStatus, 1)
	stream := make(chanStreamer, events)

	storage := &eventProvingStorage{MockStorage: &testabilities.MockStorage{}, provenPerEvent: provenPerEvent}
	daemon, err := monitor.NewDaemon(logging.NewTestLogger(t), storage, monitor.DefaultDaemonEventOptions(
		monitor.WithProvenTxChannel(provenCh),
		monitor.WithBroadcastEventStream(stream),
	))
	require.NoError(t, err)
	require.NoError(t, daemon.Start(t.Context(), nil))

	// when: several stream events arrive while the subscriber is slow
	for height := range uint32(events) {
		stream <- minedEvent(height + 1)
	}

	received := map[string]bool{}
	for len(received) < events*provenPerEvent {
		select {
		case msg := <-provenCh:
			received[msg.TxID] = true
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out after %d of %d proven events", len(received), events*provenPerEvent)
		}
	}

	// then: every proof reached the subscriber exactly once
	assert.Len(t, received, events*provenPerEvent)
}

func TestStop_ClosingSubscriberChannelAfterStopIsSafe(t *testing.T) {
	t.Parallel()

	// given: a daemon still producing proven events when it is stopped
	provenCh := make(chan wdk.CurrentTxStatus, 1)
	stream := make(chanStreamer, 10)

	storage := &eventProvingStorage{MockStorage: &testabilities.MockStorage{}, provenPerEvent: 100}
	daemon, err := monitor.NewDaemon(logging.NewTestLogger(t), storage, monitor.DefaultDaemonEventOptions(
		monitor.WithProvenTxChannel(provenCh),
		monitor.WithBroadcastEventStream(stream),
	))
	require.NoError(t, err)
	require.NoError(t, daemon.Start(t.Context(), nil))

	var wg sync.WaitGroup
	received := 0
	wg.Go(func() {
		for range provenCh {
			received++
		}
	})

	for height := range uint32(10) {
		stream <- minedEvent(height + 1)
	}

	// when: the owner stops the daemon and then closes the channel, as infra.Server does
	require.NoError(t, daemon.Stop(context.Background()))

	// then: nothing sends anymore, so closing cannot panic
	assert.NotPanics(t, func() { close(provenCh) })
	wg.Wait()

	// and: every event produced before Stop was delivered
	assert.Zero(t, received%100, "events are delivered per stream event in full")
}

func TestStop_HonoursCallerContextWhileSubscriberIsNotReading(t *testing.T) {
	t.Parallel()

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()

	tests := map[string]struct {
		newCtx func(t *testing.T) context.Context
	}{
		"already cancelled context": {
			newCtx: func(*testing.T) context.Context { return cancelled },
		},
		"short deadline": {
			newCtx: func(t *testing.T) context.Context {
				ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
				t.Cleanup(cancel)
				return ctx
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// given: a proven event backlog nobody is reading
			provenCh := make(chan wdk.CurrentTxStatus, 1)
			stream := make(chanStreamer, 1)

			storage := &eventProvingStorage{MockStorage: &testabilities.MockStorage{}, provenPerEvent: 100}
			daemon, err := monitor.NewDaemon(logging.NewTestLogger(t), storage, monitor.DefaultDaemonEventOptions(
				monitor.WithProvenTxChannel(provenCh),
				monitor.WithBroadcastEventStream(stream),
			))
			require.NoError(t, err)
			require.NoError(t, daemon.Start(t.Context(), nil))

			stream <- minedEvent(1)
			require.Eventually(t, func() bool { return len(provenCh) == 1 }, 5*time.Second, 10*time.Millisecond)

			// when: the daemon is stopped with a context that ends long before the default drain timeout
			stopped := make(chan error, 1)
			go func() { stopped <- daemon.Stop(tt.newCtx(t)) }()

			// then: Stop gives up on the backlog instead of waiting for the subscriber
			select {
			case err := <-stopped:
				require.NoError(t, err)
			case <-time.After(5 * time.Second):
				t.Fatal("Stop did not honour the caller's context")
			}

			// and: nothing sends anymore, so the owner can still close the channel
			assert.NotPanics(t, func() { close(provenCh) })
		})
	}
}
