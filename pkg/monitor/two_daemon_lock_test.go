package monitor_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/monitor"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/monitor/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// slowStorage wraps a testabilities.MockStorage (which keeps atomic call
// counters) and makes the monitored SendWaitingTransactions run take a fixed,
// non-trivial amount of time. Holding the lease for the duration of the run
// makes cross-daemon exclusion deterministic instead of hinging on the
// sub-millisecond stagger between the two schedulers' start times.
type slowStorage struct {
	*testabilities.MockStorage

	delay time.Duration
}

func (s *slowStorage) SendWaitingTransactions(ctx context.Context, minTransactionAge time.Duration) (*wdk.ProcessActionResult, error) {
	time.Sleep(s.delay)
	return s.MockStorage.SendWaitingTransactions(ctx, minTransactionAge)
}

// TestTwoDaemons_LeaseAdmitsOneRunPerSlot runs two daemons over a single shared
// database with one 1s task. With a real distributed lease, exactly one daemon
// runs the job per 1s slot, so over ~3.5s the total number of runs across both
// daemons is ~3. The previous vacuous 1ms wall-clock-bucket identifier let both
// daemons run every slot (~7-8 total) because independent pods never collided
// on a key; this test fails against that behavior and passes with the lease.
func TestTwoDaemons_LeaseAdmitsOneRunPerSlot(t *testing.T) {
	db := newLockTestDB(t)
	logger := logging.NewTestLogger(t)

	storageA := &slowStorage{MockStorage: &testabilities.MockStorage{}, delay: 200 * time.Millisecond}
	storageB := &slowStorage{MockStorage: &testabilities.MockStorage{}, delay: 200 * time.Millisecond}

	daemonA, err := monitor.NewDaemonWithGORMLocker(context.Background(), logger, storageA, db)
	require.NoError(t, err)
	daemonB, err := monitor.NewDaemonWithGORMLocker(context.Background(), logger, storageB, db)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = daemonA.Stop()
		_ = daemonB.Stop()
	})

	tasksToStart := map[defs.MonitorTask]defs.TaskConfig{
		defs.SendWaitingMonitorTask: {
			Enabled:          true,
			IntervalSeconds:  1,
			StartImmediately: false,
		},
	}

	// Start back-to-back so the two schedulers' 1s grids are as close as
	// possible; whichever ticks first each slot wins the lease, the other is
	// refused while the winner's run holds it.
	require.NoError(t, daemonA.Start(context.Background(), tasksToStart))
	require.NoError(t, daemonB.Start(context.Background(), tasksToStart))

	time.Sleep(3500 * time.Millisecond)

	// Stop both before reading the counters so no run is in flight.
	require.NoError(t, daemonA.Stop())
	require.NoError(t, daemonB.Stop())

	callsA := storageA.SendWaitingTransactionsCalled.Load()
	callsB := storageB.SendWaitingTransactionsCalled.Load()
	total := callsA + callsB

	t.Logf("send_waiting runs: A=%d B=%d total=%d", callsA, callsB, total)

	require.GreaterOrEqualf(t, total, int64(2),
		"the job must actually run across the two daemons (A=%d B=%d)", callsA, callsB)
	require.LessOrEqualf(t, total, int64(5),
		"the lease must admit ~one run per 1s slot; a vacuous lock would yield ~7-8 (A=%d B=%d)", callsA, callsB)
}
