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
