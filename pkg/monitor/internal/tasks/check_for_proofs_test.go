package tasks_test

import (
	"testing"
	"time"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/monitor"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/monitor/internal/testabilities"
	"github.com/stretchr/testify/require"
)

func TestTaskTime(t *testing.T) {
	// given:
	taskInterval := time.Millisecond * 100

	daemon := testabilities.Given(t).Daemon()

	// when:
	err := daemon.Start(map[defs.MonitorTask]time.Duration{
		defs.CheckForProofsMonitorTask: taskInterval,
	})
	require.NoError(t, err)

	// and:
	activeTask, ok := daemon.Get(defs.CheckForProofsMonitorTask)
	require.True(t, ok)

	// then:
	ensureTaskExecutedInTime(t, activeTask, taskInterval)
}

func ensureTaskExecutedInTime(t testing.TB, activeTask *monitor.ActiveTask, taskInterval time.Duration) {
	zeroTime := time.Time{}
	timeout := time.Now().Add(5 * taskInterval)
	for time.Now().Before(timeout) {
		lastRun, err := activeTask.Cronjob.LastRun()
		require.NoError(t, err)

		if lastRun == zeroTime {
			time.Sleep(taskInterval / 10)
			continue
		}

		require.True(t, lastRun.After(time.Now().Add(-2*taskInterval)))
		return
	}
	t.Fatal("scheduled task was not called in time")
}
