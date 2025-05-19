package tasks_test

import (
	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestTaskTime(t *testing.T) {
	// given:
	taskInterval := time.Millisecond * 100

	activeStorage := testabilities.Given(t).
		Provider().
		GORM()

	// when:
	err := activeStorage.Monitor.Start(map[defs.MonitorTask]time.Duration{
		defs.CheckForProofsMonitorTask: taskInterval,
	})
	require.NoError(t, err)

	time.Sleep(3 * taskInterval)

	activeTask, ok := activeStorage.Monitor.Get(defs.CheckForProofsMonitorTask)
	require.True(t, ok)

	// then:
	lastRun, err := activeTask.Cronjob.LastRun()
	require.NoError(t, err)
	require.True(t, lastRun.After(time.Now().Add(-2*taskInterval)))
}
