package actions

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/go-softwarelab/common/pkg/slices"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/history"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// ReproveHeader re-proves every known transaction whose stored proof points at a block
// orphaned by a reorg (TypeScript WalletStorageManager.reproveHeader). Each transaction ends
// up in one bucket:
//   - updated: services return a proof in another block whose root the chain tracker
//     accepts; the new proof is stored.
//   - unchanged: services still return the proof in the orphaned block.
//   - unavailable: no proof, or a proof that does not validate against the chain tracker.
//
// Unchanged and unavailable transactions keep their stored proof; the caller decides whether
// to retry later.
func (p *process) ReproveHeader(ctx context.Context, orphanedHash string) (*wdk.ReproveHeaderResult, error) {
	txIDs, err := p.knownTxRepo.FindKnownTxIDsByBlockHash(ctx, orphanedHash)
	if err != nil {
		return nil, fmt.Errorf("failed to find known txs for orphaned block %s: %w", orphanedHash, err)
	}

	result := &wdk.ReproveHeaderResult{}
	for _, txID := range txIDs {
		if err = ctx.Err(); err != nil {
			return nil, fmt.Errorf("context canceled, aborting reprove of block %s: %w", orphanedHash, err)
		}

		var updated *wdk.TxSynchronizedStatus
		updated, err = p.reproveTx(ctx, txID, orphanedHash, result)
		if err != nil {
			// Keep going so the proofs already updated in this block are still reported;
			// the failed tx keeps its old proof and the caller retries the block.
			p.logger.ErrorContext(ctx, "Failed to reprove reorged tx", slog.String("txID", txID), slog.Any("err", err))
			result.Unavailable = append(result.Unavailable, txID)
			continue
		}
		if updated != nil {
			result.Updated = append(result.Updated, *updated)
		}
	}

	p.logger.InfoContext(
		ctx, "Reproved transactions of orphaned block",
		slog.String("orphaned_hash", orphanedHash),
		slog.Int("impacted", len(txIDs)),
		slog.Int("updated", len(result.Updated)),
		slog.Int("unchanged", len(result.Unchanged)),
		slog.Int("unavailable", len(result.Unavailable)),
	)

	return result, nil
}

// reproveTx classifies a single transaction. It records unchanged/unavailable outcomes in
// result and returns the new status when the proof was updated.
func (p *process) reproveTx(ctx context.Context, txID, orphanedHash string, result *wdk.ReproveHeaderResult) (*wdk.TxSynchronizedStatus, error) {
	logger := p.logger.With(slog.String("txID", txID), slog.String("orphaned_hash", orphanedHash))

	merkleResult, err := p.services.MerklePath(ctx, txID)
	if err != nil || merkleResult.MerklePath == nil || merkleResult.BlockHeader == nil {
		if err != nil && !errors.Is(err, wdk.ErrNotFoundError) {
			logger.WarnContext(ctx, "Failed to fetch merkle path for reorged tx", slog.Any("err", err))
		}
		logger.DebugContext(ctx, "Merkle path update unavailable")
		result.Unavailable = append(result.Unavailable, txID)
		return nil, nil
	}

	header := merkleResult.BlockHeader
	if header.Hash == orphanedHash {
		logger.DebugContext(ctx, "Merkle path update still based on the orphaned block")
		result.Unchanged = append(result.Unchanged, txID)
		return nil, nil
	}

	merkleRoot, err := p.validatedMerkleRoot(ctx, merkleResult.MerklePath, txID)
	if err != nil {
		logger.WarnContext(ctx, "Merkle path update failed validation", slog.Any("err", err))
		result.Unavailable = append(result.Unavailable, txID)
		return nil, nil
	}

	transactionIDs, err := p.txRepo.FindTransactionIDsByTxID(ctx, txID)
	if err != nil {
		return nil, fmt.Errorf("failed to find transaction IDs by txID %s: %w", txID, err)
	}

	err = p.knownTxRepo.UpdateKnownTxAsMined(ctx, &entity.KnownTxAsMined{
		TxID:        txID,
		BlockHeight: merkleResult.MerklePath.BlockHeight,
		MerklePath:  merkleResult.MerklePath.Bytes(),
		BlockHash:   header.Hash,
		MerkleRoot:  merkleRoot,
		Notes: slices.Map(transactionIDs, func(transactionID uint) history.Builder {
			return history.NewBuilder().NotifyTxOfProof(transactionID)
		}),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update reproved known tx %s: %w", txID, err)
	}

	reference, labels, err := p.referenceAndLabelsForTxID(ctx, txID)
	if err != nil {
		return nil, err
	}

	logger.InfoContext(
		ctx, "Reorged tx proof updated",
		slog.String("new_block_hash", header.Hash),
		slog.Any("new_block_height", merkleResult.MerklePath.BlockHeight),
	)

	return &wdk.TxSynchronizedStatus{
		TxID:        txID,
		Status:      wdk.ProvenTxStatusCompleted,
		Reference:   reference,
		BlockHeight: merkleResult.MerklePath.BlockHeight,
		BlockHash:   header.Hash,
		MerklePath:  merkleResult.MerklePath,
		MerkleRoot:  merkleRoot,
		Labels:      labels,
	}, nil
}
