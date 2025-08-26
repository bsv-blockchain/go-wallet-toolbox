package tasks

import (
	"context"
	"fmt"
)

type AbandonedTransactionsAborter interface {
	AbortAbandoned(ctx context.Context) error
}

type FailAbandonedTask struct {
	storage AbandonedTransactionsAborter
}

func NewFailAbandonedTask(storage AbandonedTransactionsAborter) *FailAbandonedTask {
	return &FailAbandonedTask{
		storage: storage,
	}
}

func (t *FailAbandonedTask) Run(ctx context.Context) error {
	if err := t.storage.AbortAbandoned(ctx); err != nil {
		return fmt.Errorf("abort abandoned transactions failed: %w", err)
	}

	return nil
}
