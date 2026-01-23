package tasks

import (
	"context"
	"fmt"
)

// UnFailChecker checks failed transactions against the chain and updates their status if they are found.
type UnFailChecker interface {
	UnFail(ctx context.Context) error
}

// UnFailTask iterates failed transactions and re-checks their on-chain status.
type UnFailTask struct {
	storage UnFailChecker
}

func NewUnFailTask(storage UnFailChecker) TaskInterface {
	return &UnFailTask{storage: storage}
}

func (t *UnFailTask) Run(ctx context.Context) error {
	if err := t.storage.UnFail(ctx); err != nil {
		return fmt.Errorf("check failed transactions failed: %w", err)
	}
	return nil
}
