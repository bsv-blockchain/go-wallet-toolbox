package tasks_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/monitor/internal/tasks"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/monitor/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
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
	// pass nil channel and nil logger to match new constructor signature; task will return early because channel is nil
	task := tasks.NewSendWaitingTask(mockStorage, nil, nil)

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
	task := tasks.NewSendWaitingTask(mockStorage, broadcasted, logging.NewTestLogger(t))

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
