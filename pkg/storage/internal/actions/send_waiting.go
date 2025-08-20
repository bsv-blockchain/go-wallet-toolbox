package actions

import (
	"context"
	"fmt"
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

	paging := queryopts.Paging{Limit: sendWaitingItemsPerPage, Sort: "asc"}
	since := queryopts.Since{
		Time: time.Now().Add(agedLimit),
	}

	var txIDsToBroadcast []string
	batchesToBroadcast := make(map[string][]string)

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

		for _, item := range txIDsPage {
			if item.Batch != nil {
				batchesToBroadcast[*item.Batch] = append(batchesToBroadcast[*item.Batch], item.TxID)
			} else {
				txIDsToBroadcast = append(txIDsToBroadcast, item.TxID)
			}
		}

		paging.Next()
	}

	if len(txIDsToBroadcast) == 0 && len(batchesToBroadcast) == 0 {
		log.InfoContext(ctx, "No transactions found to send")
		return nil
	}

	log.InfoContext(ctx, "Found transactions to send", "transactions count", len(txIDsToBroadcast), "batches count", len(batchesToBroadcast))

	for _, txID := range txIDsToBroadcast {
		p.delayedBroadcastTransaction(ctx, log, []string{txID})
	}

	for batchName, txIDs := range batchesToBroadcast {
		log.InfoContext(ctx, "Processing batch", "batchName", batchName, "txIDs", txIDs)

		if len(txIDs) == 0 {
			log.WarnContext(ctx, "No transactions found in batch", "batchName", batchName)
			continue
		}

		p.delayedBroadcastTransaction(ctx, log, txIDs)
	}

	// TODO: Keep in mind that the transactions above max attempts will be reviewed in another "reviewStatus"

	return nil
}

func (p *process) delayedBroadcastTransaction(ctx context.Context, log *slog.Logger, txIDs []string) {
	log.InfoContext(ctx, "Attempting to broadcast transactions", "txIDs", txIDs)

	if err := p.knownTxRepo.IncreaseKnownTxAttemptsForTxIDs(ctx, txIDs); err != nil {
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
			log.WarnContext(ctx, "Problematic broadcast result", "txID", res.TxID, "status", res.Status)
		}
	}

	if !success {
		log.WarnContext(ctx, "Broadcasting transactions failed", "txIDs", txIDs)
	}

	log.InfoContext(ctx, "Successfully broadcasted transactions", "txIDs", txIDs)
}
