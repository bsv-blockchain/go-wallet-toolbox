package tasks_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/eventqueue"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/monitor/internal/tasks"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/monitor/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

func TestSendWaitingMonitorTask(t *testing.T) {
	t.Parallel()
	// given:
	given, then := testabilities.New(t)

	const seconds = 1
	taskInterval := seconds * time.Second

	daemon := given.Daemon()

	// when:
	err := daemon.Start(t.Context(), map[defs.MonitorTask]defs.TaskConfig{
		defs.SendWaitingMonitorTask: {
			Enabled:          true,
			IntervalSeconds:  seconds,
			StartImmediately: false,
		},
	})
	require.NoError(t, err)

	// then:
	then.SendWaitingTransactions().
		WaitForTaskExecution(taskInterval).
		ExecutedInTime().
		Called()
}

func TestSendWaitingMonitorTask_StartedImmediately(t *testing.T) {
	t.Parallel()
	// given:
	given, then := testabilities.New(t)

	daemon := given.Daemon()

	// when:
	err := daemon.Start(t.Context(), map[defs.MonitorTask]defs.TaskConfig{
		defs.SendWaitingMonitorTask: {
			Enabled:          true,
			IntervalSeconds:  1,
			StartImmediately: true,
		},
	})
	require.NoError(t, err)

	// then:
	then.SendWaitingTransactions().
		WaitForTaskExecution(100 * time.Millisecond).
		ExecutedInTime().
		Called()
}

func TestSendWaitingMonitorTask_FirstRunWithZeroMinTransactionAge(t *testing.T) {
	t.Parallel()
	// given:
	mockStorage := &testabilities.MockStorage{}
	// pass a nil publisher; the task skips building events because nobody subscribed
	task := tasks.NewSendWaitingTask(mockStorage, nil)

	// when:
	err := task.Run(t.Context())
	require.NoError(t, err, "task should run without error")

	// then:
	require.EqualValues(t, 1, mockStorage.SendWaitingTransactionsCalled.Load())
	require.Equal(t, time.Duration(0), mockStorage.SendWaitingLastMinTransactionAge)

	// when:
	err = task.Run(t.Context())

	// then:
	require.NoError(t, err, "task should run without error on subsequent call")
	require.EqualValues(t, 2, mockStorage.SendWaitingTransactionsCalled.Load())
	require.NotZero(t, mockStorage.SendWaitingLastMinTransactionAge)
}

func TestSendWaitingMonitorTask_ForwardsBroadcastedResultsToChannel(t *testing.T) {
	t.Parallel()
	// given: a task wired to a real (buffered) TxBroadcasted channel and a storage mock that
	// returns a non-empty result. This exercises the forwarding path that used to be dead code
	// (storage always returned nil, so results.NotDelayedResults never flowed to the channel).
	mockStorage := &testabilities.MockStorage{}
	broadcasted := make(chan wdk.CurrentTxStatus, 4)
	events := eventqueue.New[wdk.CurrentTxStatus]("test", broadcasted, logging.NewTestLogger(t))
	defer events.Discard()
	task := tasks.NewSendWaitingTask(mockStorage, events)

	// when:
	err := task.Run(t.Context())

	// then:
	require.NoError(t, err)
	require.EqualValues(t, 1, mockStorage.SendWaitingTransactionsCalled.Load())

	// and: the canned NotDelayedResults entry is forwarded onto the channel as a TxBroadcasted message.
	select {
	case msg := <-broadcasted:
		assert.Equal(t, testabilities.CannedSendWaitingTxID, msg.TxID)
		assert.Equal(t, wdk.ReviewActionResultStatusSuccess.ToStandardizedStatus(), msg.Status)
		assert.Equal(t, "canned-reference", msg.Reference)
	case <-time.After(2 * time.Second):
		t.Fatal("expected a TxBroadcasted message to be forwarded to the channel, but none arrived")
	}
}

type burstSender struct {
	count int
}

func (b burstSender) SendWaitingTransactions(context.Context, time.Duration) (*wdk.ProcessActionResult, error) {
	results := make([]wdk.ReviewActionResult, 0, b.count)
	for i := range b.count {
		results = append(results, wdk.ReviewActionResult{
			TxID:   primitives.TXIDHexString(fmt.Sprintf("%064x", i)),
			Status: wdk.ReviewActionResultStatusSuccess,
		})
	}
	return &wdk.ProcessActionResult{NotDelayedResults: results}, nil
}

func TestSendWaitingMonitorTask_BurstLargerThanChannelIsNotDropped(t *testing.T) {
	t.Parallel()
	// given: a subscriber channel much smaller than the number of results, and nobody reading
	const count = 1000
	broadcasted := make(chan wdk.CurrentTxStatus, 10)
	events := eventqueue.New[wdk.CurrentTxStatus]("test", broadcasted, logging.NewTestLogger(t))
	defer events.Discard()
	task := tasks.NewSendWaitingTask(burstSender{count: count}, events)

	// when:
	err := task.Run(t.Context())

	// then: the task finished without waiting for the subscriber
	require.NoError(t, err)

	// and: every result reaches the subscriber, in order
	for i := range count {
		select {
		case msg := <-broadcasted:
			require.Equal(t, fmt.Sprintf("%064x", i), msg.TxID)
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out waiting for event %d", i)
		}
	}
}
