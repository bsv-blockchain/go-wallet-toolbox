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
	activeStorage := testabilities.Given(t).
		Provider().
		GORM()

	// when:
	err := activeStorage.Monitor.Start(map[defs.MonitorTask]time.Duration{
		defs.CheckForProofsMonitorTask: time.Millisecond * 100,
	})

	// then:
	require.NoError(t, err)

	// when:
	time.Sleep(time.Millisecond * 500)
	activeTask, ok := activeStorage.Monitor.Get(defs.CheckForProofsMonitorTask)
	require.True(t, ok)

	// and:
	lastRun, err := activeTask.Cronjob.LastRun()
	require.NoError(t, err)
	require.True(t, lastRun.After(time.Now().Add(-time.Millisecond*500)))
}
