package tasks

import (
	"context"
	"fmt"
	"time"
)

type WaitingTransactionsSender interface {
	SendWaitingTransactions(ctx context.Context, agedLimit time.Duration) error
}

type SendWaitingTask struct {
	storage  WaitingTransactionsSender
	firstRun bool
}

func NewSendWaitingTask(storage WaitingTransactionsSender) TaskInterface {
	return &SendWaitingTask{
		storage:  storage,
		firstRun: true,
	}
}

func (t *SendWaitingTask) Run(ctx context.Context) error {
	if err := t.storage.SendWaitingTransactions(ctx, t.agedLimit()); err != nil {
		return fmt.Errorf("send waiting transactions failed: %w", err)
	}

	return nil
}

func (t *SendWaitingTask) agedLimit() time.Duration {
	if t.firstRun {
		t.firstRun = false
		return 0
	}
	return 5 * time.Minute
}
