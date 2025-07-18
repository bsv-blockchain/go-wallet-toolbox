package arc

import (
	"context"
	"fmt"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/history"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/is"
	"github.com/go-softwarelab/common/pkg/to"
)

func (s *Service) MerklePath(ctx context.Context, txID string) (*wdk.MerklePathResult, error) {
	txInfo, err := s.queryTransaction(ctx, txID)
	if err != nil {
		return nil, fmt.Errorf("arc query tx %s failed: %w", txID, err)
	}

	if txInfo == nil {
		return nil, fmt.Errorf("tx %s not found", txID)
	}

	if txInfo.TxID != txID {
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
	blockHeight, err := to.UInt32(txInfo.BlockHeight)
	if err != nil {
		return nil, fmt.Errorf("invalid block height %d in merkle path %s for transaction %s: %w", txInfo.BlockHeight, txInfo.MerklePath, txInfo.TxID, err)
	}

	if merklePath.BlockHeight != blockHeight {
		return nil, fmt.Errorf("merkle path %s block height %d does not match tx block height %d", txInfo.MerklePath, merklePath.BlockHeight, txInfo.BlockHeight)
	}

	merkleRoot, err := merklePath.ComputeRootHex(&txInfo.TxID)
	if err != nil {
		return nil, fmt.Errorf("failed to compute block hash from merkle path %s root for tx %s: %w", txInfo.MerklePath, txInfo.TxID, err)
	}

	return &wdk.MerklePathResult{
		Name:       ServiceName,
		MerklePath: merklePath,
		BlockHeader: &wdk.MerklePathBlockHeader{
			Height:     blockHeight,
			Hash:       txInfo.BlockHash,
			MerkleRoot: merkleRoot,
		},
		Notes: history.NewBuilder().GetMerklePathSuccess(ServiceName).Note().AsList(),
	}, nil
}
