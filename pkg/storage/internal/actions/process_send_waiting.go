package actions

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/tracing"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

const (
	sendWaitingMaxPages     = 10
	sendWaitingItemsPerPage = 1000
)

var (
	statusesOfWaitingTxs = []wdk.ProvenTxReqStatus{
		wdk.ProvenTxStatusUnsent,
		wdk.ProvenTxStatusSending,
	}
)

func (p *process) SendWaitingTransactions(ctx context.Context, minTransactionAge time.Duration) (*wdk.ProcessActionResult, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "StorageActions-SendWaitingTransactions")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	log := p.logger.With("action", "sendWaitingTransactions").With(slog.Duration("minTransactionAge", minTransactionAge))
	log.InfoContext(ctx, "Attempting to send waiting transactions")

	lockAcquired := p.sendWaitingLock.TryLock()
	if !lockAcquired {
		log.Warn("SendWaitingTransactions is already running, skipping this run")
		return nil, nil
	}
	defer p.sendWaitingLock.Unlock()

	paging := queryopts.Paging{Limit: sendWaitingItemsPerPage, Sort: "asc"}
	until := queryopts.Until{
		Time: time.Now().Add(-minTransactionAge),
	}

	batchesToBroadcast := make(map[string][]string)

	for range sendWaitingMaxPages {
		txIDsPage, err := p.knownTxRepo.FindKnownTxIDsByStatuses(
			ctx,
			statusesOfWaitingTxs,
			queryopts.WithUntil(until),
			queryopts.WithPage(paging),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to find known txs by statuses: %w", err)
		}

		for _, item := range txIDsPage {
			if item.Batch != nil {
				batchesToBroadcast[*item.Batch] = append(batchesToBroadcast[*item.Batch], item.TxID)
			} else {
				batchesToBroadcast[item.TxID] = []string{item.TxID}
			}
		}

		if len(txIDsPage) < sendWaitingItemsPerPage {
			break
		}

		paging.Next()
	}

	if len(batchesToBroadcast) == 0 {
		log.InfoContext(ctx, "No transactions found to send")
		return nil, nil
	}

	log.InfoContext(ctx, "Found transactions to send", "batchesCount", len(batchesToBroadcast))

	results := &wdk.ProcessActionResult{}
	for batchName, txIDs := range batchesToBroadcast {
		log.InfoContext(ctx, "Processing batch", "batchName", batchName, "txIDs", txIDs)

		res := p.broadcastDelayedTransaction(ctx, log, txIDs)
		if res != nil {
			results.SendWithResults = append(results.SendWithResults, res.SendWithResults...)
			results.NotDelayedResults = append(results.NotDelayedResults, res.NotDelayedResults...)
		}
	}

	// TODO: Keep in mind that the transactions above max attempts will be reviewed in another "reviewStatus" periodic task.

	return nil, nil
}

func (p *process) broadcastDelayedTransaction(ctx context.Context, log *slog.Logger, txIDs []string) *wdk.ProcessActionResult {
	log.InfoContext(ctx, "Attempting to broadcast transactions", "txIDs", txIDs)

	result, err := p.broadcastTxs(ctx, txIDs, false)
	if err != nil {
		log.ErrorContext(ctx, "Failed to broadcast transaction", "txIDs", txIDs, "error", err)
		return nil
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

	return result
}
