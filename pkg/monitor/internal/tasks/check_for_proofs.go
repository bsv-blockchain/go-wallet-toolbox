package tasks

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type TransactionStatusesSynchronizer interface {
	SynchronizeTransactionStatuses(ctx context.Context) ([]wdk.TxSynchronizedStatus, error)
}

type CheckForProofsTask struct {
	storage              TransactionStatusesSynchronizer
	communicationChannel chan<- defs.MonitorTaskResponse
	logger               *slog.Logger
}

func NewCheckForProofsTask(storage TransactionStatusesSynchronizer, communicationChannel chan<- defs.MonitorTaskResponse, log *slog.Logger) TaskInterface {
	return &CheckForProofsTask{
		storage:              storage,
		communicationChannel: communicationChannel,
		logger:               log,
	}
}

func (t *CheckForProofsTask) Run(ctx context.Context) error {
	results, err := t.storage.SynchronizeTransactionStatuses(ctx)
	if err != nil {
		return fmt.Errorf("synchronize transaction statuses failed: %w", err)
	}

	if t.communicationChannel == nil {
		return nil
	}

	for _, res := range results {
		msg := defs.MonitorTaskResponse{
			TxID:        res.TxID,
			Status:      string(res.Status),
			MerkleRoot:  res.MerkleRoot,
			MerklePath:  res.MerklePath,
			BlockHeight: res.BlockHeight,
			BlockHash:   res.BlockHash,
		}

		select {
		case t.communicationChannel <- msg:
		case <-ctx.Done():
			return ctx.Err()
		default:
			t.logger.Warn("TxBroadcasted channel full, dropping event")
		}
	}

	return nil
}
