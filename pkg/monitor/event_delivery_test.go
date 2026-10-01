package monitor_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/bsv-blockchain/go-chaintracks/chaintracks"
	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/monitor"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/monitor/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// tipProvingStorage reports provenPerTip proven transactions for every new tip.
type tipProvingStorage struct {
	*testabilities.MockStorage

	provenPerTip int
}

func (s *tipProvingStorage) ProcessNewTip(_ context.Context, height uint32, _ string) ([]wdk.TxSynchronizedStatus, error) {
	results := make([]wdk.TxSynchronizedStatus, 0, s.provenPerTip)
	for i := range s.provenPerTip {
		results = append(results, wdk.TxSynchronizedStatus{
			TxID:        fmt.Sprintf("tx-%d-%d", height, i),
			Status:      wdk.ProvenTxStatusCompleted,
			BlockHeight: height,
		})
	}
	return results, nil
}

func TestProvenEvents_SlowSubscriberMissesNothing(t *testing.T) {
	t.Parallel()

	// given: a proven channel with room for one event and a burst of proofs per tip
	const tips, provenPerTip = 5, 200
	provenCh := make(chan wdk.CurrentTxStatus, 1)
	tipCh := make(chan *chaintracks.BlockHeader, tips)

	storage := &tipProvingStorage{MockStorage: &testabilities.MockStorage{}, provenPerTip: provenPerTip}
	daemon, err := monitor.NewDaemon(logging.NewTestLogger(t), storage, monitor.DefaultDaemonEventOptions(
		monitor.WithProvenTxChannel(provenCh),
		monitor.WithTipChannel(tipCh),
	))
	require.NoError(t, err)
	require.NoError(t, daemon.Start(t.Context(), nil))

	// when: several tips arrive while the subscriber is slow
	for height := range uint32(tips) {
		tipCh <- &chaintracks.BlockHeader{Height: height + 1, Hash: chainhash.Hash{byte(height + 1)}}
	}

	received := map[string]bool{}
	for len(received) < tips*provenPerTip {
		select {
		case msg := <-provenCh:
			received[msg.TxID] = true
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out after %d of %d proven events", len(received), tips*provenPerTip)
		}
	}

	// then: every proof reached the subscriber exactly once
	assert.Len(t, received, tips*provenPerTip)
}

func TestStop_ClosingSubscriberChannelAfterStopIsSafe(t *testing.T) {
	t.Parallel()

	// given: a daemon still producing proven events when it is stopped
	provenCh := make(chan wdk.CurrentTxStatus, 1)
	tipCh := make(chan *chaintracks.BlockHeader, 10)

	storage := &tipProvingStorage{MockStorage: &testabilities.MockStorage{}, provenPerTip: 100}
	daemon, err := monitor.NewDaemon(logging.NewTestLogger(t), storage, monitor.DefaultDaemonEventOptions(
		monitor.WithProvenTxChannel(provenCh),
		monitor.WithTipChannel(tipCh),
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
		tipCh <- &chaintracks.BlockHeader{Height: height + 1}
	}

	// when: the owner stops the daemon and then closes the channel, as infra.Server does
	require.NoError(t, daemon.Stop(context.Background()))

	// then: nothing sends anymore, so closing cannot panic
	assert.NotPanics(t, func() { close(provenCh) })
	wg.Wait()

	// and: every event produced before Stop was delivered
	assert.Zero(t, received%100, "events are delivered per tip in full")
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
			tipCh := make(chan *chaintracks.BlockHeader, 1)

			storage := &tipProvingStorage{MockStorage: &testabilities.MockStorage{}, provenPerTip: 100}
			daemon, err := monitor.NewDaemon(logging.NewTestLogger(t), storage, monitor.DefaultDaemonEventOptions(
				monitor.WithProvenTxChannel(provenCh),
				monitor.WithTipChannel(tipCh),
			))
			require.NoError(t, err)
			require.NoError(t, daemon.Start(t.Context(), nil))

			tipCh <- &chaintracks.BlockHeader{Height: 1}
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
