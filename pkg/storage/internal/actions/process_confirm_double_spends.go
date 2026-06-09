package actions

import (
	"context"
	"log/slog"
	"slices"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

const (
	confirmDoubleSpendAttempts  = 3
	confirmDoubleSpendRetryWait = time.Second
)

// confirmDoubleSpends re-verifies every aggregated double-spend verdict before it becomes
// a terminal failure, because failing a transaction releases its inputs back to spendable -
// done on a false positive, this lets the wallet build REAL double spends afterwards.
//
// A single broadcaster can report a false double spend: e.g. WhatsOnChain answers
// "missing inputs" for a freshly-chained tx whose parent has not reached its mempool yet,
// or reports a conflict while another service has already accepted the very same tx.
//
// The verdict is downgraded as follows:
//   - the network status check knows the tx -> the report was a false positive -> success
//   - some broadcaster accepted the tx -> transient disagreement -> serviceError (retried later)
//   - only "missing inputs" hints without competing txs -> no positive evidence of a
//     double spend (indistinguishable from propagation lag) -> serviceError (retried later)
//   - otherwise the double spend is considered confirmed and stays terminal.
//
// This mirrors confirmDoubleSpend in the TypeScript wallet-toolbox
// (src/storage/methods/attemptToPostReqsToNetwork.ts).
func (p *process) confirmDoubleSpends(ctx context.Context, aggregated wdk.AggregatedPostFromBEEF) {
	var doubtful []string
	for txID, agg := range aggregated {
		if agg.Status == wdk.AggregatedPostedTxIDDoubleSpend {
			doubtful = append(doubtful, txID)
		}
	}
	if len(doubtful) == 0 {
		return
	}
	slices.Sort(doubtful)

	knownTxs := p.checkTxsKnownToNetwork(ctx, doubtful)

	for _, txID := range doubtful {
		agg := aggregated[txID]

		logger := p.logger.With(
			slog.String("txID", txID),
			slog.Int("successCount", agg.SuccessCount),
			slog.Int("doubleSpendCount", agg.DoubleSpendCount),
		)

		switch {
		case knownTxs[txID]:
			logger.InfoContext(ctx, "Double spend reported by a broadcaster but the tx is known to the network - treating broadcast as successful")
			agg.Status = wdk.AggregatedPostedTxIDSuccess
		case agg.SuccessCount > 0:
			logger.InfoContext(ctx, "Broadcasters disagree on double spend - keeping the tx retryable instead of failing it")
			agg.Status = wdk.AggregatedPostedTxIDServiceError
		case !hasDoubleSpendEvidence(agg):
			logger.InfoContext(ctx, "Double spend hint without evidence (missing inputs may be propagation lag) - keeping the tx retryable instead of failing it")
			agg.Status = wdk.AggregatedPostedTxIDServiceError
		default:
			logger.WarnContext(ctx, "Double spend confirmed - failing the tx")
		}
	}
}

// checkTxsKnownToNetwork reports which of the given txs the network already knows
// (mempool or mined), retrying a few times because a just-broadcast tx needs a moment
// to propagate.
func (p *process) checkTxsKnownToNetwork(ctx context.Context, txIDs []string) map[string]bool {
	knownTxs := make(map[string]bool, len(txIDs))
	remaining := txIDs

	for attempt := 0; attempt < confirmDoubleSpendAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return knownTxs
			case <-time.After(confirmDoubleSpendRetryWait):
			}
		}

		statusResult, err := p.services.GetStatusForTxIDs(ctx, remaining)
		if err != nil {
			p.logger.WarnContext(ctx, "Failed to check tx statuses while verifying a double spend report",
				slog.String("error", err.Error()))
			continue
		}
		if statusResult == nil || statusResult.Status != wdk.GetStatusSuccess {
			continue
		}

		for _, detail := range statusResult.Results {
			if detail.Status != wdk.ResultStatusForTxIDNotFound.String() {
				knownTxs[detail.TxID] = true
			}
		}

		remaining = slices.DeleteFunc(slices.Clone(remaining), func(txID string) bool {
			return knownTxs[txID]
		})
		if len(remaining) == 0 {
			break
		}
	}

	return knownTxs
}

// hasDoubleSpendEvidence reports whether any broadcaster provided positive evidence of a
// double spend: a mempool-conflict-style rejection or competing transactions. A bare
// "missing inputs" rejection carries no such evidence - for chained transactions it is
// most often just the parent not having propagated to that service's mempool yet.
func hasDoubleSpendEvidence(agg *wdk.AggregatedPostedTxID) bool {
	for _, result := range agg.TxIDResults {
		if !result.DoubleSpend {
			continue
		}
		if result.Result != wdk.PostedTxIDResultMissingInputs || len(result.CompetingTxs) > 0 {
			return true
		}
	}
	return false
}
