package actions

import (
	"context"
	"fmt"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"log/slog"
	"time"
)

const (
	sendWaitingMaxPages     = 100
	sendWaitingItemsPerPage = 100
)

var (
	statusesOfWaitingTxs = []wdk.ProvenTxReqStatus{
		wdk.ProvenTxStatusUnsent,
		wdk.ProvenTxStatusSending,
	}
)

func (p *process) SendWaitingTransactions(ctx context.Context, agedLimit time.Duration) error {
	log := p.logger.With("action", "send waiting transactions").With("agedLimit", agedLimit.Seconds())
	log.InfoContext(ctx, "Attempting to send waiting transactions")

	lockAcquired := p.sendWaitingLock.TryLock()
	if !lockAcquired {
		log.Warn("SendWaitingTransactions is already running, skipping this run")
		return nil
	}
	defer p.sendWaitingLock.Unlock()

	since := queryopts.Since{
		Time: time.Now().Add(-agedLimit),
	}
	var txsToBroadcast []*entity.KnownTxForStatusSync
	paging := queryopts.Paging{Limit: sendWaitingItemsPerPage, Sort: "asc"}
	for range sendWaitingMaxPages {
		txIDsPage, err := p.knownTxRepo.FindKnownTxIDsByStatuses(
			ctx,
			statusesOfWaitingTxs,
			queryopts.WithSince(since),
			queryopts.WithPage(paging),
		)
		if err != nil {
			return fmt.Errorf("failed to find known txs by statuses: %w", err)
		}

		if len(txIDsPage) == 0 {
			break
		}

		txsToBroadcast = append(txsToBroadcast, txIDsPage...)
		paging.Next()
	}

	if len(txsToBroadcast) == 0 {
		log.InfoContext(ctx, "No transactions found to send")
		return nil
	}

	log.InfoContext(ctx, "Found transactions to send", "count", len(txsToBroadcast))

	processedBatches := map[string]struct{}{}

	for _, transaction := range txsToBroadcast {
		if transaction.Batch != nil {
			if _, exists := processedBatches[*transaction.Batch]; exists {
				log.DebugContext(ctx, "Skipping already processed batch", "batch", *transaction.Batch)
				continue
			}
			processedBatches[*transaction.Batch] = struct{}{}
		}

		p.delayedBroadcastTransaction(ctx, log, transaction)
	}

	// TODO: Keep in mind that the transactions above max attempts will be reviewed in another "reviewStatus"

	return nil
}

func (p *process) delayedBroadcastTransaction(ctx context.Context, log *slog.Logger, transaction *entity.KnownTxForStatusSync) {
	log.InfoContext(ctx, "Attempting to broadcast transaction", "txID", transaction.TxID)

	txIDs, err := p.batchedTxIDsForDelayedBroadcast(ctx, transaction)
	if err != nil {
		log.ErrorContext(ctx, "Failed to get batched tx IDs for delayed broadcast", "txID", transaction.TxID, "error", err)
		return
	}

	if err = p.knownTxRepo.IncreaseKnownTxAttemptsForTxIDs(ctx, txIDs); err != nil {
		log.ErrorContext(ctx, "Failed to increase known tx attempts", "txIDs", txIDs, "error", err)
		return
	}

	result, err := p.broadcastTxs(ctx, txIDs, false)
	if err != nil {
		log.ErrorContext(ctx, "Failed to broadcast transaction", "txIDs", txIDs, "error", err)
		return
	}

	success := true
	for _, res := range result.NotDelayedResults {
		if res.Status != wdk.ReviewActionResultStatusSuccess {
			success = false
			log.WarnContext(ctx, "Problematic broadcast result", "txID", transaction.TxID, "status", res.Status)
		}
	}

	if !success {
		log.WarnContext(ctx, "Broadcasting transactions failed", "txIDs", txIDs)
	}

	log.InfoContext(ctx, "Successfully broadcasted transactions", "txIDs", txIDs)
}

func (p *process) batchedTxIDsForDelayedBroadcast(ctx context.Context, transaction *entity.KnownTxForStatusSync) ([]string, error) {
	if transaction.Batch == nil {
		return []string{transaction.TxID}, nil
	}
	txIDs, err := p.knownTxRepo.FindKnownTxIDsByBatch(ctx, *transaction.Batch)
	if err != nil {
		return nil, fmt.Errorf("failed to find known txs by batch %s: %w", *transaction.Batch, err)
	}

	return txIDs, nil
}
