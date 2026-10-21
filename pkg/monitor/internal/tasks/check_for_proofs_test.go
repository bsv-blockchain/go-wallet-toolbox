package tasks_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/monitor/internal/tasks"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/monitor/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
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

type partialSyncStorage struct {
	results []wdk.TxSynchronizedStatus
	err     error
}

func (s partialSyncStorage) SynchronizeTransactionStatuses(context.Context) ([]wdk.TxSynchronizedStatus, error) {
	return s.results, s.err
}

type recordingPublisher struct {
	published []wdk.CurrentTxStatus
}

func (p *recordingPublisher) Publish(msg wdk.CurrentTxStatus) {
	p.published = append(p.published, msg)
}

func TestCheckForProofsPublishesCommittedResultsWhenSyncFails(t *testing.T) {
	t.Parallel()
	// given:
	syncErr := errors.New("context deadline exceeded")
	storage := partialSyncStorage{
		results: []wdk.TxSynchronizedStatus{
			{TxID: "tx-1", Status: wdk.ProvenTxStatusCompleted},
			{TxID: "tx-2", Status: wdk.ProvenTxStatusCompleted},
		},
		err: syncErr,
	}
	publisher := &recordingPublisher{}
	task := tasks.NewCheckForProofsTask(storage, publisher)

	// when:
	err := task.Run(t.Context())

	// then:
	require.ErrorIs(t, err, syncErr)
	require.Len(t, publisher.published, 2)
	require.Equal(t, "tx-1", publisher.published[0].TxID)
	require.Equal(t, "tx-2", publisher.published[1].TxID)
}
