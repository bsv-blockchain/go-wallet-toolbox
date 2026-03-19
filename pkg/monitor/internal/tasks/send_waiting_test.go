package tasks_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/monitor/internal/tasks"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/monitor/internal/testabilities"
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
	require.Equal(t, 1, mockStorage.SendWaitingTransactionsCalled)
	require.Equal(t, time.Duration(0), mockStorage.SendWaitingLastMinTransactionAge)

	// when:
	err = task.Run(t.Context())

	// then:
	require.NoError(t, err, "task should run without error on subsequent call")
	require.Equal(t, 2, mockStorage.SendWaitingTransactionsCalled)
	require.NotZero(t, mockStorage.SendWaitingLastMinTransactionAge)
}
