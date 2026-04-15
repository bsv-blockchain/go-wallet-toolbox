package testabilities

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
)

type MonitorAssertions interface {
	SynchronizeTransactionStatuses() ExecutedTaskAssertions
	SendWaitingTransactions() ExecutedTaskAssertions
	FailAbandoned() ExecutedTaskAssertions
}

type ExecutedTaskAssertions interface {
	WaitForTaskExecution(expectedInterval time.Duration) TaskExecutionAssertions
}

type TaskExecutionAssertions interface {
	Called() TaskExecutionAssertions
	ExecutedInTime() TaskExecutionAssertions
}

type monitorAssertions struct {
	t        testing.TB
	fixtures *monitorFixture
	require  *require.Assertions
}

func then(t testing.TB, fixture *monitorFixture) *monitorAssertions {
	return &monitorAssertions{
		t:        t,
		fixtures: fixture,
		require:  require.New(t),
	}
}

func (m *monitorAssertions) SynchronizeTransactionStatuses() ExecutedTaskAssertions {
	return &storageMethodAssertions{
		called: func() int {
			return m.fixtures.mockStorage.SynchronizeTransactionStatusesCalled
		},
		taskName: defs.CheckForProofsMonitorTask,
		parent:   m,
	}
}

func (m *monitorAssertions) SendWaitingTransactions() ExecutedTaskAssertions {
	return &storageMethodAssertions{
		called: func() int {
			return m.fixtures.mockStorage.SendWaitingTransactionsCalled
		},
		taskName: defs.SendWaitingMonitorTask,
		parent:   m,
	}
}

func (m *monitorAssertions) FailAbandoned() ExecutedTaskAssertions {
	return &storageMethodAssertions{
		called: func() int {
			return m.fixtures.mockStorage.FailAbandonedCalled
		},
		taskName: defs.FailAbandonedMonitorTask,
		parent:   m,
	}
}

type storageMethodAssertions struct {
	called   func() int
	parent   *monitorAssertions
	taskName defs.MonitorTask
	interval time.Duration
	lastRun  *time.Time
}

func (s *storageMethodAssertions) Called() TaskExecutionAssertions {
	s.parent.t.Helper()

	s.parent.require.Eventuallyf(func() bool {
		return s.called() > 0
	}, 5*time.Second, s.interval, "expected SynchronizeTransactionStatuses to be called: %s", s.taskName)

	return s
}

func (s *storageMethodAssertions) WaitForTaskExecution(expectedInterval time.Duration) TaskExecutionAssertions {
	s.parent.t.Helper()
	s.parent.require.NotNil(s.parent.fixtures.daemon, "Expected daemon to be initialized")

	s.interval = expectedInterval

	activeTask, ok := s.parent.fixtures.daemon.Get(s.taskName)
	s.parent.require.True(ok, "Expected daemon task to be initialized")

	timeoutDuration := 5 * expectedInterval
	timeout := time.Now().Add(timeoutDuration)
	for time.Now().Before(timeout) {
		lastRun, err := activeTask.Cronjob.LastRunStartedAt()
		s.parent.require.NoError(err)

		if lastRun.IsZero() {
			time.Sleep(expectedInterval / 10)
			continue
		}

		s.lastRun = &lastRun
		return s
	}
	s.parent.require.FailNow("scheduled task was not called - timeout reached", timeoutDuration)

	return s
}

func (s *storageMethodAssertions) ExecutedInTime() TaskExecutionAssertions {
	s.parent.t.Helper()
	s.parent.require.NotNil(s.parent.fixtures.daemon, "Expected daemon to be initialized")

	s.parent.require.NotNil(s.lastRun, "Expected lastRun to be initialized")
	// Use 5*interval tolerance to match WaitForTaskExecution's timeout and avoid
	// flakes on loaded CI runners where goroutine scheduling delays can push
	// lastRun past a tighter window even though the task ran correctly.
	s.parent.require.True(s.lastRun.After(time.Now().Add(-5*s.interval)), "scheduled task lastRun is not within the expected interval")

	return s
}
