package monitor

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
)

// budgetCapturingTask records the deadline its run was given.
type budgetCapturingTask struct {
	ran         bool
	deadline    time.Time
	hasDeadline bool
	err         error
}

func (c *budgetCapturingTask) Run(ctx context.Context) error {
	c.ran = true
	c.deadline, c.hasDeadline = ctx.Deadline()
	return c.err
}

func TestContextWithTimeout(t *testing.T) {
	d := &Daemon{logger: logging.NewTestLogger(t)}

	t.Run("budgets the interval less the safety margin", func(t *testing.T) {
		ctx, cancel := d.contextWithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		deadline, ok := ctx.Deadline()
		require.True(t, ok)
		assert.InDelta(t, float64(285*time.Second), float64(time.Until(deadline)), float64(time.Second))
	})

	t.Run("an unset interval leaves the context alone", func(t *testing.T) {
		for _, interval := range []time.Duration{0, -time.Second} {
			ctx, cancel := d.contextWithTimeout(context.Background(), interval)
			cancel()

			_, ok := ctx.Deadline()
			assert.False(t, ok, "interval %s must not produce a deadline", interval)
		}
	})

	t.Run("an earlier parent deadline still wins", func(t *testing.T) {
		parent, cancelParent := context.WithTimeout(context.Background(), time.Second)
		defer cancelParent()

		ctx, cancel := d.contextWithTimeout(parent, time.Hour)
		defer cancel()

		deadline, ok := ctx.Deadline()
		require.True(t, ok)
		assert.Less(t, time.Until(deadline), 2*time.Second)
	})
}

// TestSingleTaskRunner_BudgetsFromTheInterval is the regression guard for the
// production incident: gocron started send_waiting 17.85ms BEFORE its scheduled
// instant, NextRun() still reported that instant rather than the following one, and
// the run was handed a ~17ms budget instead of 285s. Every one of the 197 batches it
// had just discovered then failed on its first query, and the whole run lasted 34ms.
//
// Deriving the budget from the configured interval makes that impossible by
// construction, whatever the scheduler reports.
func TestSingleTaskRunner_BudgetsFromTheInterval(t *testing.T) {
	d := &Daemon{logger: logging.NewTestLogger(t)}
	task := &budgetCapturingTask{}

	// Cronjob is deliberately nil: the budget must not depend on the scheduler being
	// able to answer questions about the next run.
	runner := d.singleTaskRunner(&ActiveTask{
		Instance: task,
		TaskName: defs.SendWaitingMonitorTask,
		Interval: 5 * time.Minute,
	})
	runner(context.Background())

	require.True(t, task.ran, "the task must run even when the scheduler cannot report a next run")
	require.True(t, task.hasDeadline)
	assert.InDelta(t, float64(285*time.Second), float64(time.Until(task.deadline)), float64(time.Second))
}

func TestSingleTaskRunner_ReportsTaskFailure(t *testing.T) {
	d := &Daemon{logger: logging.NewTestLogger(t)}
	task := &budgetCapturingTask{err: errors.New("boom: task failed")}

	runner := d.singleTaskRunner(&ActiveTask{
		Instance: task,
		TaskName: defs.SendWaitingMonitorTask,
		Interval: time.Minute,
	})

	assert.NotPanics(t, func() { runner(context.Background()) })
	assert.True(t, task.ran)
}

// TestInitializeTaskRecordsTheInterval pins the wiring the budget depends on: an
// ActiveTask built by the daemon must carry the configured interval.
func TestInitializeTaskRecordsTheInterval(t *testing.T) {
	d, err := NewDaemon(logging.NewTestLogger(t), nil, DefaultDaemonEventOptions())
	require.NoError(t, err)

	require.NoError(t, d.initializeTask(
		&budgetCapturingTask{},
		defs.SendWaitingMonitorTask,
		defs.TaskConfig{Enabled: true, IntervalSeconds: 300},
	))

	active, ok := d.Get(defs.SendWaitingMonitorTask)
	require.True(t, ok)
	assert.Equal(t, 5*time.Minute, active.Interval)
}
