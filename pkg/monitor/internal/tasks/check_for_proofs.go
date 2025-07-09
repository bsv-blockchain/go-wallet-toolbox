package tasks

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
)

type TransactionStatusesSynchronizer interface {
	SynchronizeTransactionStatuses(ctx context.Context) error
}

type CheckForProofsTask struct {
	logger  *slog.Logger
	storage TransactionStatusesSynchronizer
}

func NewCheckForProofsTask(logger *slog.Logger, storage TransactionStatusesSynchronizer) TaskInterface {
	return &CheckForProofsTask{
		logger:  logging.Child(logger, "check_for_proofs"),
		storage: storage,
	}
}

func (t *CheckForProofsTask) Run(ctx context.Context) error {
	if err := t.storage.SynchronizeTransactionStatuses(ctx); err != nil {
		return fmt.Errorf("synchronize transaction statuses failed: %w", err)
	}

	return nil
}
