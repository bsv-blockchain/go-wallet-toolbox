package tasks

import (
	"context"
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type TransactionStatusesSynchronizer interface {
	SynchronizeTransactionStatuses(ctx context.Context) ([]wdk.TxSynchronizedStatus, error)
}

// CheckForProofsTask periodically pulls merkle proofs via storage status sync.
// When Arcade is enabled this is a fallback: the preferred path is the Arcade
// SSE broadcast-event handler, which pushes proofs without polling.
type CheckForProofsTask struct {
	storage        TransactionStatusesSynchronizer
	txProvenEvents StatusPublisher
}

func NewCheckForProofsTask(storage TransactionStatusesSynchronizer, txProvenEvents StatusPublisher) TaskInterface {
	return &CheckForProofsTask{
		storage:        storage,
		txProvenEvents: txProvenEvents,
	}
}

func (t *CheckForProofsTask) Run(ctx context.Context) error {
	results, err := t.storage.SynchronizeTransactionStatuses(ctx)
	if err != nil {
		return fmt.Errorf("synchronize transaction statuses failed: %w", err)
	}

	if t.txProvenEvents == nil {
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

		t.txProvenEvents.Publish(msg)
	}

	return nil
}
