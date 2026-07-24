package arcade

import (
	"context"
	"fmt"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/go-softwarelab/common/pkg/is"
	"go.opentelemetry.io/otel/attribute"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/history"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/tracing"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// MerklePath polls Arcade GET /tx/{txID} for a merkle proof.
//
// This is a fallback only. The preferred path is the Arcade SSE /events stream
// (BroadcastStatusEvents / monitor broadcast-event handler), which pushes status
// and merkle proofs without polling. MerklePath is used by status sync and
// check_for_proofs when SSE is down, lagging, or not wired.
//
// Behavior for the service queue:
//   - 404 / not known to Arcade → error so the next MerklePath provider is tried
//   - known but not yet proven → empty success (no failover; same as classic ARC)
//   - mined with a valid merklePath → success
func (s *Service) MerklePath(ctx context.Context, txID string) (_ *wdk.MerklePathResult, err error) {
	ctx, span := tracing.StartTracing(ctx, "Services-MerklePath", attribute.String("service", "arcade"))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	txInfo, err := s.QueryTx(ctx, txID)
	if err != nil {
		return nil, fmt.Errorf("arcade query tx %s failed: %w", txID, err)
	}
	if txInfo == nil {
		return nil, fmt.Errorf("tx %s not found", txID)
	}
	if txInfo.TxID != "" && txInfo.TxID != txID {
		return nil, fmt.Errorf("got response for tx %s while querying for %s", txInfo.TxID, txID)
	}

	if is.BlankString(txInfo.MerklePath) {
		return &wdk.MerklePathResult{
			Name:  ServiceName,
			Notes: history.NewBuilder().GetMerklePathNotFound(ServiceName).Note().AsList(),
		}, nil
	}

	merklePath, err := transaction.NewMerklePathFromHex(txInfo.MerklePath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse merkle path %s: %w", txInfo.MerklePath, err)
	}

	if merklePath.BlockHeight != txInfo.BlockHeight {
		return nil, fmt.Errorf(
			"merkle path %s block height %d does not match tx block height %d",
			txInfo.MerklePath, merklePath.BlockHeight, txInfo.BlockHeight,
		)
	}

	merkleRoot, err := merklePath.ComputeRootHex(&txID)
	if err != nil {
		return nil, fmt.Errorf("failed to compute block hash from merkle path %s root for tx %s: %w", txInfo.MerklePath, txID, err)
	}

	return &wdk.MerklePathResult{
		Name:       ServiceName,
		MerklePath: merklePath,
		BlockHeader: &wdk.MerklePathBlockHeader{
			Height:     txInfo.BlockHeight,
			Hash:       txInfo.BlockHash,
			MerkleRoot: merkleRoot,
		},
		Notes: history.NewBuilder().GetMerklePathSuccess(ServiceName).Note().AsList(),
	}, nil
}
