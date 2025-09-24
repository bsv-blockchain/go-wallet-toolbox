package tasks

import (
	"context"
	"fmt"
)

type TransactionStatusesSynchronizer interface {
	SynchronizeTransactionStatuses(ctx context.Context) error
}

// UnFailChecker checks failed transactions against the chain and updates their status if they are found.
type UnFailChecker interface {
	UnFail(ctx context.Context) error
}

type CheckForProofsTask struct {
	storage TransactionStatusesSynchronizer
}

func NewCheckForProofsTask(storage TransactionStatusesSynchronizer) TaskInterface {
	return &CheckForProofsTask{
		storage: storage,
	}
}

func (t *CheckForProofsTask) Run(ctx context.Context) error {
	if err := t.storage.SynchronizeTransactionStatuses(ctx); err != nil {
		return fmt.Errorf("synchronize transaction statuses failed: %w", err)
	}

	return nil
}
