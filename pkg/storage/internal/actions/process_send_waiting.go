package actions

import (
	"context"
	"errors"
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

var statusesOfWaitingTxs = []wdk.ProvenTxReqStatus{
	wdk.ProvenTxStatusUnsent,
	wdk.ProvenTxStatusSending,
}

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
		log.WarnContext(ctx, "SendWaitingTransactions is already running, skipping this run")
		return nil, nil
	}
	defer p.sendWaitingLock.Unlock()

	paging := queryopts.Paging{Limit: sendWaitingItemsPerPage, Sort: "asc"}
	until := queryopts.Until{
		Time: time.Now().Add(-minTransactionAge),
	}

	batchesToBroadcast := make(map[string][]string)

	for range sendWaitingMaxPages {
		txIDsPage, pageErr := p.knownTxRepo.FindKnownTxIDsByStatuses(
			ctx,
			statusesOfWaitingTxs,
			queryopts.WithUntil(until),
			queryopts.WithPage(paging),
		)
		if pageErr != nil {
			err = fmt.Errorf("failed to find known txs by statuses: %w", pageErr)
			return nil, err
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
	var batchErrs []error
	for batchName, txIDs := range batchesToBroadcast {
		log.InfoContext(ctx, "Processing batch", "batchName", batchName, "txIDs", txIDs)

		res, batchErr := p.broadcastDelayedTransaction(ctx, log, txIDs)
		if batchErr != nil {
			// Continue-on-error: a single bad batch must not strand the rest. Record the error and
			// keep going; the aggregated (joined) error is returned to the caller at the end.
			log.ErrorContext(ctx, "Failed to broadcast waiting batch", "batchName", batchName, "txIDs", txIDs, "error", batchErr)
			batchErrs = append(batchErrs, batchErr)
			continue
		}
		if res != nil {
			results.SendWithResults = append(results.SendWithResults, res.SendWithResults...)
			results.NotDelayedResults = append(results.NotDelayedResults, res.NotDelayedResults...)
		}
	}

	// TODO: Keep in mind that the transactions above max attempts will be reviewed in another "reviewStatus" periodic task.

	// Return the assembled result together with any hard batch errors (joined). Soft, per-tx
	// failures (e.g. a service error that leaves a tx still "sending") are reported inside the
	// result's per-tx entries, not as a returned error.
	err = errors.Join(batchErrs...)
	return results, err
}

// broadcastDelayedTransaction broadcasts a single waiting batch and returns its result and error
// instead of logging them away. A hard failure (broadcastTxs returns an error) is surfaced to the
// caller so it can be aggregated; per-tx problematic outcomes are left in the returned result.
func (p *process) broadcastDelayedTransaction(ctx context.Context, log *slog.Logger, txIDs []string) (*wdk.ProcessActionResult, error) {
	log.InfoContext(ctx, "Attempting to broadcast transactions", "txIDs", txIDs)

	// Storage-wide sweep: it runs for no particular user, so a pre-broadcast abort is not
	// limited to a single owner's rows.
	result, err := p.broadcastTxs(ctx, txIDs, false, nil)
	if err != nil {
		return nil, fmt.Errorf("broadcast of waiting batch %v failed: %w", txIDs, err)
	}

	for _, res := range result.NotDelayedResults {
		if res.Status != wdk.ReviewActionResultStatusSuccess {
			log.WarnContext(ctx, "Problematic broadcast result", "txID", res.TxID, "status", res.Status)
		}
	}

	return result, nil
}
