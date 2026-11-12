package actions

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/go-softwarelab/common/pkg/seq"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/txutils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// broadcastLookups holds the per-txID data broadcastTxs needs, read once up front.
type broadcastLookups struct {
	knownTxStatuses map[string]wdk.ProvenTxReqStatus
	txReferences    map[string]string
	txLabels        map[string][]string
}

func (p *process) loadBroadcastLookups(ctx context.Context, txIDs []string) (broadcastLookups, error) {
	knownTxStatuses, err := p.getKnownTxStatuses(ctx, txIDs...)
	if err != nil {
		return broadcastLookups{}, err
	}

	txReferences, err := p.txRepo.FindReferencesByTxIDs(ctx, txIDs)
	if err != nil {
		return broadcastLookups{}, fmt.Errorf("failed to find references for txIDs: %w", err)
	}

	txLabels, err := p.txRepo.GetLabelsForTxIDs(ctx, txIDs)
	if err != nil {
		return broadcastLookups{}, fmt.Errorf("failed to find labels for txIDs: %w", err)
	}

	return broadcastLookups{
		knownTxStatuses: knownTxStatuses,
		txReferences:    txReferences,
		txLabels:        txLabels,
	}, nil
}

// buildVerifiedBEEF builds the BEEF for the transactions about to be sent and verifies their scripts.
func (p *process) buildVerifiedBEEF(ctx context.Context, logger *slog.Logger, readyToSendTxIDs []string) (*transaction.Beef, error) {
	logger.DebugContext(
		ctx, "Building BEEF for ready-to-send transactions",
		slog.Int("readyToSendCount", len(readyToSendTxIDs)),
	)

	// Direct sources only: this BEEF feeds script verification and EF
	// construction, both of which need just each input's source output. A full
	// ancestry build re-validated every ancestor BUMP root per transaction —
	// at high TPS that was the single largest CPU cost in the storage server
	// (shared fuel parents re-validated for every spend).
	beef, err := p.knownTxRepo.GetBEEFForTxIDs(
		ctx, seq.FromSlice(readyToSendTxIDs),
		entity.WithStatusesToFilterOut(wdk.ProvenTxReqProblematicStatuses...),
		entity.WithDirectSourcesOnly(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to build valid BEEF: %w", err)
	}

	logger.DebugContext(
		ctx, "Verifying built EF",
		slog.Int("readyToSendCount", len(readyToSendTxIDs)),
	)

	// Hydrate only the subject txs' direct inputs: the direct-sources-only
	// BEEF deliberately omits grandparents, and neither script verification
	// nor EF construction needs them.
	if err = txutils.HydrateBEEFSubjects(beef, readyToSendTxIDs); err != nil {
		return nil, fmt.Errorf("failed to hydrate beef for script verification: %w", err)
	}

	if err = p.verifyScriptsForReadyTxs(ctx, beef, readyToSendTxIDs); err != nil {
		return nil, err
	}

	return beef, nil
}

// queueDelayed hands the transactions to the background broadcaster instead of posting them.
// NOTE: the delayed path does NOT bump attempts. The tx is only being QUEUED for a later
// broadcast; the attempt is counted when it is actually posted (see BackgroundBroadcast).
func (p *process) queueDelayed(
	ctx context.Context,
	logger *slog.Logger,
	readyToSendTxIDs []string,
	beef *transaction.Beef,
	sendWithResults []wdk.SendWithResult,
) (*wdk.ProcessActionResult, error) {
	logger.DebugContext(
		ctx, "Processing delayed transactions",
		slog.Int("readyToSendCount", len(readyToSendTxIDs)),
	)

	resultsForDelayedTxs, err := p.processDelayedTransactions(ctx, readyToSendTxIDs, beef)
	if err != nil {
		return nil, fmt.Errorf("failed to process delayed transactions: %w", err)
	}

	return &wdk.ProcessActionResult{
		SendWithResults: append(sendWithResults, resultsForDelayedTxs...),
	}, nil
}

// claimForPost claims the transactions for broadcast and returns the ones that were actually
// claimed, in the order they have to be posted in.
func (p *process) claimForPost(ctx context.Context, logger *slog.Logger, readyToSendTxIDs []string, beef *transaction.Beef) ([]string, error) {
	logger.DebugContext(
		ctx, "Claiming transactions for broadcast",
		slog.Int("readyToSendCount", len(readyToSendTxIDs)),
	)

	claimedTxIDs, err := p.knownTxRepo.ClaimKnownTxsForBroadcast(ctx, readyToSendTxIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to claim txs for broadcast: %w", err)
	}
	if len(claimedTxIDs) != len(readyToSendTxIDs) {
		// Whatever could not be claimed was aborted or moved on in the meantime; posting it
		// anyway could put a transaction on the network whose inputs are spendable again.
		logger.WarnContext(
			ctx, "some transactions are no longer claimable for broadcast and will not be posted",
			slog.Int("readyToSendCount", len(readyToSendTxIDs)),
			slog.Int("claimedCount", len(claimedTxIDs)),
		)
	}
	if len(claimedTxIDs) == 0 {
		return nil, nil
	}
	// Keep the caller's order rather than adopting the claim's: the sweep hands these over
	// oldest-first, and the claim query returns whatever order the database happens to use.
	claimedInOrder := retainClaimed(readyToSendTxIDs, claimedTxIDs)

	// Parents ahead of their children within this post. PostFromBEEF walks the slice in order
	// and Arcade forwards upstream in receive order, so a child posted before the parent it
	// spends is rejected by Teranode. The reordering is stable, so the creation order above
	// survives everywhere there is no dependency to respect.
	return txutils.ParentsFirst(beef, claimedInOrder), nil
}

// postClaimed posts the claimed transactions and aggregates the per-tx outcome of the post
// over all txIDs of the call.
func (p *process) postClaimed(
	ctx context.Context,
	logger *slog.Logger,
	txIDs, toPostTxIDs []string,
	beef *transaction.Beef,
) (map[string]*wdk.AggregatedPostedTxID, map[string]error, error) {
	// Count the attempt only for the txs we are ACTUALLY posting now (toPostTxIDs), right
	// before the post. Already-sent txs (which took the early return / sendWithResults branch) and
	// delayed/queued txs are intentionally NOT bumped here, so attempts reflect real broadcast
	// attempts and proof-timeout logic fires on true post counts.
	logger.DebugContext(
		ctx, "Increasing attempt counters for transactions being posted",
		slog.Int("readyToSendCount", len(toPostTxIDs)),
	)

	if err := p.knownTxRepo.IncreaseKnownTxAttemptsForTxIDs(ctx, toPostTxIDs); err != nil {
		return nil, nil, fmt.Errorf("failed to increase known tx attempts: %w", err)
	}

	logger.DebugContext(
		ctx, "Posting BEEF to services",
		slog.Int("readyToSendCount", len(toPostTxIDs)),
	)

	results, err := p.services.PostFromBEEF(ctx, beef, toPostTxIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to post BEEF: %w", err)
	}

	aggregated := results.Aggregated(txIDs)
	p.confirmDoubleSpends(ctx, aggregated)

	return aggregated, results.ServiceErrors(), nil
}
