package actions

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/go-softwarelab/common/pkg/seq"
	"gorm.io/gorm"

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

// broadcastClaim is the outcome of claimForPost. Under group commit the claim and the attempts
// bump of postClaimed share one unit of work; attemptsCounted then records that the bump already
// ran and attemptsErr its outcome, so postClaimed reports it at the same point as before.
type broadcastClaim struct {
	toPostTxIDs     []string
	attemptsCounted bool
	attemptsErr     error
}

// claimForPost claims the transactions for broadcast and returns the ones that were actually
// claimed, in the order they have to be posted in.
func (p *process) claimForPost(ctx context.Context, logger *slog.Logger, readyToSendTxIDs []string, beef *transaction.Beef) (broadcastClaim, error) {
	logger.DebugContext(
		ctx, "Claiming transactions for broadcast",
		slog.Int("readyToSendCount", len(readyToSendTxIDs)),
	)

	claim, claimedTxIDs, err := p.claimKnownTxs(ctx, readyToSendTxIDs)
	if err != nil {
		return broadcastClaim{}, fmt.Errorf("failed to claim txs for broadcast: %w", err)
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
		return broadcastClaim{}, nil
	}
	// Keep the caller's order rather than adopting the claim's: the sweep hands these over
	// oldest-first, and the claim query returns whatever order the database happens to use.
	claimedInOrder := retainClaimed(readyToSendTxIDs, claimedTxIDs)

	// Parents ahead of their children within this post. PostFromBEEF walks the slice in order
	// and Arcade forwards upstream in receive order, so a child posted before the parent it
	// spends is rejected by Teranode. The reordering is stable, so the creation order above
	// survives everywhere there is no dependency to respect.
	claim.toPostTxIDs = txutils.ParentsFirst(beef, claimedInOrder)
	return claim, nil
}

// claimKnownTxs claims the transactions for broadcast. Under group commit it also bumps their
// attempts in the same unit of work - the bump postClaimed would otherwise run on its own right
// after - inside a nested savepoint, so that a failed bump leaves the claim committed exactly as
// the ungrouped path does. The bump covers the claimed set, which is the set postClaimed bumps.
func (p *process) claimKnownTxs(ctx context.Context, readyToSendTxIDs []string) (broadcastClaim, []string, error) {
	var (
		claim        broadcastClaim
		claimedTxIDs []string
	)

	solo := func(ctx context.Context) error {
		claim = broadcastClaim{}
		var err error
		claimedTxIDs, err = p.knownTxRepo.ClaimKnownTxsForBroadcast(ctx, readyToSendTxIDs)
		return err
	}

	var err error
	if p.group == nil {
		err = solo(ctx)
	} else {
		err = p.group.Do(ctx, func(ctx context.Context, scope GroupScope) error {
			claim, claimedTxIDs = broadcastClaim{}, nil

			knownTxRepo := scope.Repos().KnownTxRepo()
			claimed, claimErr := knownTxRepo.ClaimKnownTxsForBroadcast(ctx, readyToSendTxIDs)
			if claimErr != nil {
				return claimErr
			}
			claimedTxIDs = claimed
			if len(claimed) == 0 {
				return nil
			}

			// The repository runs on tx's connection, so its statement lands inside the savepoint.
			claim.attemptsCounted = true
			claim.attemptsErr = scope.Tx.WithContext(ctx).Transaction(func(*gorm.DB) error {
				return knownTxRepo.IncreaseKnownTxAttemptsForTxIDs(ctx, claimed)
			})
			return nil
		}, solo)
	}
	if err != nil {
		return broadcastClaim{}, nil, err
	}
	return claim, claimedTxIDs, nil
}

// postClaimed posts the claimed transactions and aggregates the per-tx outcome of the post
// over all txIDs of the call.
func (p *process) postClaimed(
	ctx context.Context,
	logger *slog.Logger,
	txIDs []string,
	claim broadcastClaim,
	beef *transaction.Beef,
) (map[string]*wdk.AggregatedPostedTxID, map[string]error, error) {
	toPostTxIDs := claim.toPostTxIDs

	// Count the attempt only for the txs we are ACTUALLY posting now (toPostTxIDs), right
	// before the post. Already-sent txs (which took the early return / sendWithResults branch) and
	// delayed/queued txs are intentionally NOT bumped here, so attempts reflect real broadcast
	// attempts and proof-timeout logic fires on true post counts.
	logger.DebugContext(
		ctx, "Increasing attempt counters for transactions being posted",
		slog.Int("readyToSendCount", len(toPostTxIDs)),
	)

	attemptsErr := claim.attemptsErr
	if !claim.attemptsCounted {
		attemptsErr = p.knownTxRepo.IncreaseKnownTxAttemptsForTxIDs(ctx, toPostTxIDs)
	}
	if attemptsErr != nil {
		return nil, nil, fmt.Errorf("failed to increase known tx attempts: %w", attemptsErr)
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
