package actions

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/tracing"
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
	var err error
	ctx, span := tracing.StartTracing(ctx, "StorageActions-AbortAbandoned")
	defer func() {
		tracing.EndTracing(span, err)
	}()

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
		if err := a.outputsRepo.ShouldTxOutputsBeUnspent(ctx, id); err != nil {
			msg := "This might indicate a SERIOUS problem with the storage consistency! Cannot abort transaction because some outputs are already spent."
			log.ErrorContext(ctx, msg, logging.Number("transactionID", id), logging.Error(err))
			continue
		}

		if err := a.abortTx(ctx, id); err != nil {
			log.ErrorContext(ctx, "Failed to abort transaction", logging.Number("transactionID", id), logging.Error(err))
		} else {
			log.InfoContext(ctx, "Successfully aborted transaction", logging.Number("transactionID", id))
		}
	}

	return nil
}
