package tasks_test

import (
	"testing"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/monitor/internal/testabilities"
	"github.com/stretchr/testify/require"
)

func TestFailAbandoned(t *testing.T) {
	t.Parallel()
	// given:
	given, then := testabilities.New(t)

	const seconds = 1
	taskInterval := seconds * time.Second

	daemon := given.Daemon()

	// when:
	err := daemon.Start(map[defs.MonitorTask]defs.TaskConfig{
		defs.FailAbandonedMonitorTask: {
			Enabled:          true,
			IntervalSeconds:  seconds,
			StartImmediately: false,
		},
	})
	require.NoError(t, err)

	// then:
	then.FailAbandoned().
		WaitForTaskExecution(taskInterval).
		ExecutedInTime().
		Called()
}
