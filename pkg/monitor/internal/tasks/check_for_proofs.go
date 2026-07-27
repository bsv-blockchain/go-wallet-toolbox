package tasks

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type TransactionStatusesSynchronizer interface {
	SynchronizeTransactionStatuses(ctx context.Context) ([]wdk.TxSynchronizedStatus, error)
}

// CheckForProofsTask periodically pulls merkle proofs via storage status sync.
// When Arcade is enabled this is a fallback: the preferred path is the Arcade
// SSE broadcast-event handler, which pushes proofs without polling.
type CheckForProofsTask struct {
	storage         TransactionStatusesSynchronizer
	txProvenChannel chan<- wdk.CurrentTxStatus
	logger          *slog.Logger
}

func NewCheckForProofsTask(storage TransactionStatusesSynchronizer, txProvenChannel chan<- wdk.CurrentTxStatus, log *slog.Logger) TaskInterface {
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
		msg := wdk.CurrentTxStatus{
			TxID:        res.TxID,
			Status:      res.Status.ToStandardizedStatus(),
			MerkleRoot:  res.MerkleRoot,
			MerklePath:  res.MerklePath,
			BlockHeight: res.BlockHeight,
			BlockHash:   res.BlockHash,
			Reference:   res.Reference,
			Labels:      res.Labels,
		}

		select {
		case t.txProvenChannel <- msg:
		case <-ctx.Done():
			return fmt.Errorf("context done while sending tx status update: %w", ctx.Err())
		default:
			t.logger.WarnContext(ctx, "TxProven channel full, dropping event")
		}
	}

	return nil
}
