package arcade

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/sync/errgroup"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/tracing"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// getStatusConcurrency bounds parallel GET /tx/{txID} lookups: the status-sync
// task can ask about thousands of txIDs at once and one-by-one polling would
// make the sync window minutes long against a local Arcade.
const getStatusConcurrency = 16

// ChainTipHeightFunc returns the current chain tip height, used to compute
// confirmation depth for mined transactions.
type ChainTipHeightFunc func(ctx context.Context) (uint32, error)

// SetChainTipHeight wires the tip-height provider used to derive confirmation
// depth in GetStatusForTxIDs. Without it (or when it errors) mined
// transactions report the conservative minimum depth of 1.
func (s *Service) SetChainTipHeight(fn ChainTipHeightFunc) {
	s.chainTipHeight.Store(&fn)
}

// GetStatusForTxIDs reports mined/known/unknown status per txID from Arcade's
// GET /tx/{txID} endpoint, satisfying the same contract as the
// WhatsOnChain/Bitails implementations so status sync works on networks where
// Arcade is the only reachable service (e.g. private TSTN).
//
// Mapping:
//   - MINED / IMMUTABLE → "mined", depth = tip − blockHeight + 1 (min 1)
//   - 404 / REJECTED → "unknown" (not on chain), no depth
//   - any other lifecycle status → "known" (seen, unconfirmed), depth 0
//
// Individual lookup failures are tolerated as long as at least one lookup
// succeeds; the failed txIDs are simply absent from the results (the consumer
// skips them until the next sync round).
func (s *Service) GetStatusForTxIDs(ctx context.Context, txIDs []string) (_ *wdk.GetStatusForTxIDsResult, err error) {
	ctx, span := tracing.StartTracing(ctx, "Services-GetStatusForTxIDs", attribute.String("service", "arcade"))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	if len(txIDs) == 0 {
		return nil, fmt.Errorf("no txIDs provided")
	}

	tip := s.currentTipHeight(ctx)

	var (
		mu       sync.Mutex
		results  []wdk.TxStatusDetail
		firstErr error
	)
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(getStatusConcurrency)
	for _, txID := range txIDs {
		g.Go(func() error {
			info, queryErr := s.QueryTx(gctx, txID)

			mu.Lock()
			defer mu.Unlock()
			switch {
			case queryErr == nil:
				results = append(results, toStatusDetail(txID, info, tip))
			case errors.Is(queryErr, wdk.ErrNotFoundError):
				results = append(results, wdk.TxStatusDetail{
					TxID:   txID,
					Status: wdk.ResultStatusForTxIDNotFound.String(),
				})
			default:
				if firstErr == nil {
					firstErr = queryErr
				}
			}
			return nil // tolerate individual failures; see firstErr below
		})
	}
	_ = g.Wait()

	if len(results) == 0 {
		if firstErr != nil {
			return nil, fmt.Errorf("failed to get status for txIDs: %w", firstErr)
		}
		return nil, fmt.Errorf("no results found for provided txIDs")
	}

	return &wdk.GetStatusForTxIDsResult{
		Name:    ServiceName,
		Status:  wdk.GetStatusSuccess,
		Results: results,
	}, nil
}

// currentTipHeight resolves the chain tip, or 0 when no provider is wired or
// it fails (mined depth then falls back to the conservative minimum of 1).
func (s *Service) currentTipHeight(ctx context.Context) uint32 {
	fnPtr := s.chainTipHeight.Load()
	if fnPtr == nil || *fnPtr == nil {
		return 0
	}
	tip, err := (*fnPtr)(ctx)
	if err != nil {
		s.logger.WarnContext(ctx, "failed to resolve chain tip for status depth, using minimum depth for mined txs")
		return 0
	}
	return tip
}

// toStatusDetail maps one Arcade TXInfo to the shared status contract.
func toStatusDetail(txID string, info *TXInfo, tip uint32) wdk.TxStatusDetail {
	switch info.TxStatus {
	case StatusMined, StatusImmutable:
		depth := 1
		if tip > 0 && info.BlockHeight > 0 && tip >= info.BlockHeight {
			depth = int(tip-info.BlockHeight) + 1
		}
		return wdk.TxStatusDetail{
			TxID:   txID,
			Depth:  &depth,
			Status: wdk.ResultStatusForTxIDMined.String(),
		}
	case StatusRejected, StatusUnknown:
		return wdk.TxStatusDetail{
			TxID:   txID,
			Status: wdk.ResultStatusForTxIDNotFound.String(),
		}
	case StatusReceived, StatusSentToNetwork, StatusAcceptedByNetwork,
		StatusSeenOnNetwork, StatusSeenMultipleNodes, StatusSeenOnMultipleNodes,
		StatusDoubleSpendAttempted, StatusPendingRetry, StatusStumpProcessing,
		StatusAnnouncedToNetwork, StatusStored:
		// Seen by the network but not yet mined.
		fallthrough
	default:
		depth := 0
		return wdk.TxStatusDetail{
			TxID:   txID,
			Depth:  &depth,
			Status: wdk.ResultStatusForTxIDKnown.String(),
		}
	}
}
