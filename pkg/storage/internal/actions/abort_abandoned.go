package actions

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

const (
	failAbandonedMaxPages     = 10
	failAbandonedItemsPerPage = 1000
)

var (
	statusesOfAbandonedTxs = []wdk.TxStatus{
		wdk.TxStatusUnprocessed,
		wdk.TxStatusUnsigned,
	}
)

func (a *abortAction) AbortAbandoned(ctx context.Context, minTransactionAge time.Duration) error {
	log := a.logger.With("action", "failAbandonedTransactions").With(slog.Duration("minTransactionAge", minTransactionAge))
	log.InfoContext(ctx, "Attempting to fail abandoned transactions")

	lockAcquired := a.failAbandonedLock.TryLock()
	if !lockAcquired {
		log.Warn("FailAbandonedTransactions is already running, skipping this run")
		return nil
	}
	defer a.failAbandonedLock.Unlock()

	paging := queryopts.Paging{Limit: failAbandonedItemsPerPage, Sort: "asc"}
	until := queryopts.Until{
		Time: time.Now().Add(-minTransactionAge),
	}

	var idsToAbort []uint

	for range failAbandonedMaxPages {
		transactionIDs, err := a.transactionsRepo.FindTransactionIDsByStatuses(
			ctx,
			statusesOfAbandonedTxs,
			queryopts.WithUntil(until),
			queryopts.WithPage(paging),
		)
		if err != nil {
			return fmt.Errorf("failed to find transactions by statuses: %w", err)
		}

		idsToAbort = append(idsToAbort, transactionIDs...)

		if len(transactionIDs) < failAbandonedItemsPerPage {
			break
		}

		paging.Next()
	}

	if len(idsToAbort) == 0 {
		log.InfoContext(ctx, "No abandoned transactions found to fail")
		return nil
	}

	log.InfoContext(ctx, "Found abandoned transactions to fail", slog.Int("count", len(idsToAbort)))

	for _, id := range idsToAbort {
		if err := a.outputsRepo.IsAnyOutputOfTransactionSpent(ctx, id); err != nil {
			log.ErrorContext(ctx, "Cannot abort transaction because some outputs are already spent", "transactionID", id, "error", err.Error())
			continue
		}

		if err := a.abortTx(ctx, id); err != nil {
			log.ErrorContext(ctx, "Failed to abort transaction", "transactionID", id, "error", err.Error())
		} else {
			log.InfoContext(ctx, "Successfully aborted transaction", "transactionID", id)
		}
	}

	return nil
}
