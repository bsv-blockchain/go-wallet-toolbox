package service_test

import (
	"context"
	"fmt"
	"iter"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bsv-blockchain/go-sdk/transaction"
	testvectors "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/service"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type mockBroadcaster struct {
	called          atomic.Int64
	sleep           time.Duration
	returnErr       error
	panicDuringCall error
}

func (m *mockBroadcaster) BackgroundBroadcast(ctx context.Context, _ *transaction.Beef, _ []string) ([]wdk.ReviewActionResult, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(m.sleep):
		// Simulate a delay for the broadcast operation
	}
	m.called.Add(1)

	switch {
	case m.returnErr != nil:
		return nil, m.returnErr
	case m.panicDuringCall != nil:
		panic(m.panicDuringCall)
	default:
		return nil, nil
	}
}

func (m *mockBroadcaster) waitForBroadcastCalls(t testing.TB, count int64) {
	timeout := time.After(5 * time.Second)
	for m.called.Load() < count {
		select {
		case <-timeout:
			t.Fatalf("expected %d calls to Broadcast, but got %d", count, m.called.Load())
			return
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}
}

func broadcastItemsGenerator(length int) iter.Seq[testvectors.TransactionSpec] {
	return func(yield func(item testvectors.TransactionSpec) bool) {
		for i := 0; i < length; i++ {
			txSpec := testvectors.GivenTX().
				WithInput(100).
				WithP2PKHOutput(99).
				WithOPReturn(fmt.Sprintf("test-%d", i))

			if !yield(txSpec) {
				return
			}
		}
	}
}

func loggerForTestBroadcaster() (*slog.Logger, *logging.TestWriter) {
	stringWriter := &logging.TestWriter{}
	logger := logging.New().
		WithLevel(defs.LogLevelDebug).
		WithHandler(defs.TextHandler, stringWriter).
		Logger()

	return logger, stringWriter
}

func TestBackgroundBroadcaster_HappyPath(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		length int
	}{
		"single": {
			length: 1,
		},
		"two": {
			length: 2,
		},
		"max channel size": {
			length: service.BackgroundBroadcasterChannelSize,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			mockBroadcast := &mockBroadcaster{}

			logger, _ := loggerForTestBroadcaster()
			bb := service.NewBackgroundBroadcaster(t.Context(), logger, mockBroadcast, nil)
			bb.Start()

			for txSpec := range broadcastItemsGenerator(tt.length) {
				beef, err := transaction.NewBeefFromTransaction(txSpec.TX())
				require.NoError(t, err)

				txIDs := []string{txSpec.ID().String()}

				added := bb.Add(beef, txIDs)
				assert.True(t, added, "item should be added to broadcast channel")
			}

			mockBroadcast.waitForBroadcastCalls(t, int64(tt.length))
			bb.Stop()
		})
	}
}

func TestBackgroundBroadcaster_WhenProducerIsSlowerThanConsumer(t *testing.T) {
	mockBroadcast := &mockBroadcaster{}

	logger, _ := loggerForTestBroadcaster()
	bb := service.NewBackgroundBroadcaster(t.Context(), logger, mockBroadcast, nil)
	bb.Start()

	moreThanChannerSize := 2*service.BackgroundBroadcasterChannelSize + 1

	for txSpec := range broadcastItemsGenerator(moreThanChannerSize) {
		beef, err := transaction.NewBeefFromTransaction(txSpec.TX())
		require.NoError(t, err)

		txIDs := []string{txSpec.ID().String()}

		added := bb.Add(beef, txIDs)
		assert.True(t, added, "item should be added to broadcast channel")

		time.Sleep(time.Millisecond)
	}

	mockBroadcast.waitForBroadcastCalls(t, 1)
	bb.Stop()
}

func TestBackgroundBroadcaster_WhenProducerIsFasterThanConsumer(t *testing.T) {
	mockBroadcast := &mockBroadcaster{
		sleep: 5 * time.Second, // Simulate a very slow broadcast so channel fills before consumers drain it
	}

	logger, _ := loggerForTestBroadcaster()
	bb := service.NewBackgroundBroadcaster(t.Context(), logger, mockBroadcast, nil)
	bb.Start()

	moreThanChannerSize := 2*service.BackgroundBroadcasterChannelSize + 1

	channelIsFull := false
	for txSpec := range broadcastItemsGenerator(moreThanChannerSize) {
		beef, err := transaction.NewBeefFromTransaction(txSpec.TX())
		require.NoError(t, err)
		txIDs := []string{txSpec.ID().String()}

		added := bb.Add(beef, txIDs)
		if !added {
			channelIsFull = true
			break
		}
	}

	assert.True(t, channelIsFull, "channel should be full at some point")

	bb.Stop()
}

func TestBackgroundBroadcast_StopDuringProcessing(t *testing.T) {
	mockBroadcast := &mockBroadcaster{
		sleep: 5 * time.Second, // Long delay ensures Stop() cancels context before any broadcast completes
	}

	logger, _ := loggerForTestBroadcaster()
	bb := service.NewBackgroundBroadcaster(t.Context(), logger, mockBroadcast, nil)
	bb.Start()

	const count = 10
	for txSpec := range broadcastItemsGenerator(count) {
		beef, err := transaction.NewBeefFromTransaction(txSpec.TX())
		require.NoError(t, err)
		txIDs := []string{txSpec.ID().String()}

		added := bb.Add(beef, txIDs)
		assert.True(t, added, "item should be added to broadcast channel")
	}
	bb.Stop()
	processed := mockBroadcast.called.Load()
	require.Zero(t, processed)
}

func TestBackgroundBroadcast_BroadcasterReturnsError(t *testing.T) {
	mockBroadcast := &mockBroadcaster{
		returnErr: fmt.Errorf("broadcast error"),
	}

	logger, logsBuffer := loggerForTestBroadcaster()
	bb := service.NewBackgroundBroadcaster(t.Context(), logger, mockBroadcast, nil)
	bb.Start()

	const count = 10
	for txSpec := range broadcastItemsGenerator(count) {
		beef, err := transaction.NewBeefFromTransaction(txSpec.TX())
		require.NoError(t, err)
		txIDs := []string{txSpec.ID().String()}

		added := bb.Add(beef, txIDs)
		assert.True(t, added, "item should be added to broadcast channel")
	}

	mockBroadcast.waitForBroadcastCalls(t, count)
	assert.Contains(t, logsBuffer.String(), mockBroadcast.returnErr.Error())

	bb.Stop()
}

func TestBackgroundBroadcast_BroadcasterPanics(t *testing.T) {
	mockBroadcast := &mockBroadcaster{
		panicDuringCall: fmt.Errorf("broadcast panic"),
	}

	logger, logsBuffer := loggerForTestBroadcaster()
	bb := service.NewBackgroundBroadcaster(t.Context(), logger, mockBroadcast, nil)
	bb.Start()

	const count = 10
	for txSpec := range broadcastItemsGenerator(count) {
		beef, err := transaction.NewBeefFromTransaction(txSpec.TX())
		require.NoError(t, err)
		txIDs := []string{txSpec.ID().String()}

		added := bb.Add(beef, txIDs)
		assert.True(t, added, "item should be added to broadcast channel")
	}

	mockBroadcast.waitForBroadcastCalls(t, count)
	assert.Contains(t, logsBuffer.String(), mockBroadcast.panicDuringCall.Error())

	bb.Stop()
}
