package tasks_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/monitor/internal/testabilities"
)

func TestSynchronizeTransactionStatuses(t *testing.T) {
	t.Parallel()
	// given:
	given, then := testabilities.New(t)

	const seconds = 1
	taskInterval := seconds * time.Second

	daemon := given.Daemon()

	// when:
	err := daemon.Start(t.Context(), map[defs.MonitorTask]defs.TaskConfig{
		defs.CheckForProofsMonitorTask: {
			Enabled:          true,
			IntervalSeconds:  seconds,
			StartImmediately: false,
		},
	})
	require.NoError(t, err)

	// then:
	then.SynchronizeTransactionStatuses().
		WaitForTaskExecution(taskInterval).
		ExecutedInTime().
		Called()
}
