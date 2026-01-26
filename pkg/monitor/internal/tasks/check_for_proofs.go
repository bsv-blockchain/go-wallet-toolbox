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
	storage         TransactionStatusesSynchronizer
	txProvenChannel chan<- defs.TransactionStatusUpdate
	logger          *slog.Logger
}

func NewCheckForProofsTask(storage TransactionStatusesSynchronizer, txProvenChannel chan<- defs.TransactionStatusUpdate, log *slog.Logger) TaskInterface {
	return &CheckForProofsTask{
		storage:         storage,
		txProvenChannel: txProvenChannel,
		logger:          log,
	}
}

func (t *CheckForProofsTask) Run(ctx context.Context) error {
	results, err := t.storage.SynchronizeTransactionStatuses(ctx)
	if err != nil {
		return fmt.Errorf("synchronize transaction statuses failed: %w", err)
	}

	if t.txProvenChannel == nil {
		return nil
	}

	for _, res := range results {
		msg := defs.TransactionStatusUpdate{
			TxID:        res.TxID,
			Status:      defs.ParseTxUpdateStatusOrUnknown(string(res.Status)),
			MerkleRoot:  res.MerkleRoot,
			MerklePath:  res.MerklePath,
			BlockHeight: res.BlockHeight,
			BlockHash:   res.BlockHash,
		}

		select {
		case t.txProvenChannel <- msg:
		case <-ctx.Done():
			return fmt.Errorf("context done while sending tx status update: %w", ctx.Err())
		default:
			t.logger.Warn("TxProven channel full, dropping event")
		}
	}

	return nil
}
