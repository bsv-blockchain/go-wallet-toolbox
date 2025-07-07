package tasks_test

import (
	"testing"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/monitor/internal/testabilities"
	"github.com/stretchr/testify/require"
)

func TestTaskTime(t *testing.T) {
	// given:
	given, then := testabilities.New(t)

	taskInterval := time.Millisecond * 100

	daemon := given.Daemon()

	// when:
	err := daemon.Start(map[defs.MonitorTask]time.Duration{
		defs.CheckForProofsMonitorTask: taskInterval,
	})
	require.NoError(t, err)

	// then:
	then.SynchronizeTransactionStatuses().
		WaitForTaskExecution(taskInterval).
		ExecutedInTime().
		Called()
}
