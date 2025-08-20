package tasks_test

import (
	"testing"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/monitor/internal/testabilities"
	"github.com/stretchr/testify/require"
)

func TestSendWaitingMonitorTask(t *testing.T) {
	// given:
	given, then := testabilities.New(t)

	const seconds = 1
	taskInterval := seconds * time.Second

	daemon := given.Daemon()

	// when:
	err := daemon.Start(map[defs.MonitorTask]defs.TaskConfig{
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
	// given:
	given, then := testabilities.New(t)

	daemon := given.Daemon()

	// when:
	err := daemon.Start(map[defs.MonitorTask]defs.TaskConfig{
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
