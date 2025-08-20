package tasks

import (
	"context"
	"fmt"
)

type TransactionStatusesSynchronizer interface {
	SynchronizeTransactionStatuses(ctx context.Context) error
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
