package tasks

import (
	"context"
	"fmt"
)

// CheckFailedTransactionsTask iterates failed transactions and re-checks their on-chain status.
type CheckFailedTransactionsTask struct {
	storage FailedTransactionsChecker
}

func NewCheckFailedTransactionsTask(storage FailedTransactionsChecker) TaskInterface {
	return &CheckFailedTransactionsTask{storage: storage}
}

func (t *CheckFailedTransactionsTask) Run(ctx context.Context) error {
	if err := t.storage.CheckFailedTransactions(ctx); err != nil {
		return fmt.Errorf("check failed transactions failed: %w", err)
	}
	return nil
}
