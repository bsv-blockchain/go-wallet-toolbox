package repo

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"time"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/go-softwarelab/common/pkg/slices"
	"gorm.io/gorm"
)

const (
	maxDepthOfRecursion = 1000
)

type ProvenTxReq struct {
	db *gorm.DB
}

func NewProvenTxReqRepo(db *gorm.DB) *ProvenTxReq {
	return &ProvenTxReq{db: db}
}

func (p *ProvenTxReq) UpsertProvenTxReq(ctx context.Context, req *entity.UpsertProvenTxReq, historyNote string, historyAttrs map[string]any) error {
	err := p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return upsertProvenTxReq(tx, req, historyNote, historyAttrs)
	})

	if err != nil {
		return fmt.Errorf("failed to upsert proven tx req: %w", err)
	}
	return nil
}

func upsertProvenTxReq(db *gorm.DB, req *entity.UpsertProvenTxReq, historyNote string, historyAttrs map[string]any) error {
	var model models.ProvenTxReq
	err := db.First(&model, "tx_id = ? ", req.TxID).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("cannot upsert proven tx req: %w", err)
	}

	model.Status = req.Status // TODO: Shouldn't we check the status first? Only if it's higher than the current one, we should update it.
	model.TxID = req.TxID
	model.RawTx = req.RawTx
	model.InputBeef = req.InputBeef

	model.AddNote(time.Now(), historyNote, historyAttrs)

	return db.Save(&model).Error
}

func updateProvenTxStatus(db *gorm.DB, txID string, status wdk.ProvenTxReqStatus, historyNote string, historyAttrs map[string]any) error {
	var model models.ProvenTxReq
	err := db.Model(&model).Select("status", "history").First(&model, "tx_id = ? ", txID).Error
	if err != nil {
		return fmt.Errorf("cannot update proven tx status: %w", err)
	}

	historyAttrs["oldStatus"] = model.Status
	model.Status = status
	model.AddNote(time.Now(), historyNote, historyAttrs)

	return db.Where("tx_id = ?", txID).Updates(&model).Error
}

func (p *ProvenTxReq) FindProvenTxRawTX(ctx context.Context, txID string) ([]byte, error) {
	var model models.ProvenTxReq
	err := p.db.WithContext(ctx).
		Model(&model).
		Select("raw_tx").
		First(&model, "tx_id = ? ", txID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find proven tx raw tx: %w", err)
	}
	return model.RawTx, nil
}

func (p *ProvenTxReq) FindProvenTxStatus(ctx context.Context, txID string) (wdk.ProvenTxReqStatus, error) {
	var model models.ProvenTxReq
	err := p.db.WithContext(ctx).
		Model(&model).
		Select("status").
		Where("tx_id = ? ", txID).
		First(&model).Error
	if err != nil {
		return "", fmt.Errorf("failed to find proven tx status: %w", err)
	}
	return model.Status, nil
}

func (p *ProvenTxReq) ExistsAllProvenTxs(ctx context.Context, txIDs []string, sourceTxsStatusFilter []wdk.ProvenTxReqStatus) (bool, error) {
	var model models.ProvenTxReq
	query := p.db.WithContext(ctx).
		Model(&model).
		Select("tx_id").
		Where("tx_id IN (?) ", txIDs).
		Where("raw_tx IS NOT NULL and LENGTH(raw_tx) > 0").
		Where("input_beef IS NOT NULL and LENGTH(input_beef) > 0")

	if len(sourceTxsStatusFilter) > 0 {
		query = query.Where("status IN ? ", sourceTxsStatusFilter)
	}

	var count int64
	err := query.Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("failed to check if proven transactions exist: %w", err)
	}

	return count == int64(len(txIDs)), nil
}

func (p *ProvenTxReq) BuildValidBEEF(ctx context.Context, txID string, statusesToFilterOut []wdk.ProvenTxReqStatus) (*transaction.Beef, error) {
	beef := transaction.NewBeefV2()
	err := p.recursiveBuildValidBEEF(ctx, 0, beef, txID, statusesToFilterOut)
	if err != nil {
		return nil, fmt.Errorf("failed to build valid BEEF: %w", err)
	}

	return beef, nil
}

func (p *ProvenTxReq) recursiveBuildValidBEEF(ctx context.Context, depth int, mergeToBeef *transaction.Beef, txID string, statusesToFilterOut []wdk.ProvenTxReqStatus) error {
	if depth > maxDepthOfRecursion {
		return fmt.Errorf("max depth of recursion reached: %d", maxDepthOfRecursion)
	}

	var model models.ProvenTxReq
	query := p.db.WithContext(ctx).
		Model(&model).
		Select("raw_tx, input_beef, merkle_path")

	if len(statusesToFilterOut) > 0 {
		query = query.Where("status NOT IN ? ", statusesToFilterOut)
	}

	err := query.First(&model, "tx_id = ? ", txID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return fmt.Errorf("failed to find proven tx, raw tx and input beef for tx (id: %s): %w", txID, err)
	}

	if model.RawTx == nil || model.InputBeef == nil {
		return fmt.Errorf("raw tx or input beef is nil in transaction %s", txID)
	}

	tx, err := transaction.NewTransactionFromBytes(model.RawTx)
	if err != nil {
		return fmt.Errorf("failed to build transaction object from raw tx (id: %s): %w", txID, err)
	}

	if model.HasMerklePath() {
		merklePath, err := transaction.NewMerklePathFromBinary(model.MerklePath)
		if err != nil {
			return fmt.Errorf("failed to build merkle path from binary for tx (id: %s): %w", txID, err)
		}
		err = tx.AddMerkleProof(merklePath)
		if err != nil {
			return fmt.Errorf("failed to add merkle proof to transaction (id: %s): %w", txID, err)
		}

		_, err = mergeToBeef.MergeTransaction(tx)
		if err != nil {
			return fmt.Errorf("failed to merge transaction (id: %s) into BEEF object: %w", txID, err)
		}

		return nil
	}

	for i := range tx.Inputs {
		if len(tx.Inputs[i].SourceTXID) == 0 {
			return fmt.Errorf("input of tx (id: %s) has empty SourceTXID at index %d ", txID, i)
		}
	}

	_, err = mergeToBeef.MergeRawTx(model.RawTx, nil)
	if err != nil {
		return fmt.Errorf("failed to merge raw tx (id: %s) into BEEF object: %w", txID, err)
	}

	err = mergeToBeef.MergeBeefBytes(model.InputBeef)
	if err != nil {
		return fmt.Errorf("failed to merge input beef into BEEF object: %w", err)
	}

	var sourceTXID string
	for _, input := range tx.Inputs {
		sourceTXID = input.SourceTXID.String()
		beefTx := mergeToBeef.FindTransaction(sourceTXID)
		if beefTx == nil {
			err = p.recursiveBuildValidBEEF(ctx, depth+1, mergeToBeef, sourceTXID, statusesToFilterOut)
			if err != nil {
				return fmt.Errorf("failed to recursively find proven tx and merge into BEEF: %w", err)
			}
		}
	}

	// Result is in mergeToBeef
	return nil
}

func (p *ProvenTxReq) GetBEEFForTxIDs(ctx context.Context, txids iter.Seq[string], knownTxIDs []string, statusesToFilterOut []wdk.ProvenTxReqStatus) ([]byte, error) {
	beef := transaction.NewBeefV2()

	// TODO: handle KnownTxids properly which works in a way that for provided KnownTxids beef will do `MergeTxIDOnly` instead of recursively fetching parent transactions
	_ = knownTxIDs

	for txid := range txids {
		if beef.FindTransaction(txid) != nil {
			continue
		}
		err := p.recursiveBuildValidBEEF(ctx, 0, beef, txid, statusesToFilterOut)
		if err != nil {
			return nil, fmt.Errorf("failed for txid %s: %w", txid, err)
		}
	}

	data, err := beef.Bytes()
	if err != nil {
		return nil, fmt.Errorf("beef serialization error: %w", err)
	}

	return data, nil
}

func (p *ProvenTxReq) FindProvenTxIDsByStatuses(ctx context.Context, limit int, txStatus ...wdk.ProvenTxReqStatus) ([]*entity.ProvenTxToSync, error) {
	var rows []*models.ProvenTxReq
	err := p.db.WithContext(ctx).
		Model(&models.ProvenTxReq{}).
		Select("tx_id, status, attempts").
		Where("status IN ? ", txStatus).
		Order("created_at ASC").
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find proven tx ids by statuses: %w", err)
	}

	return slices.Map(rows, func(row *models.ProvenTxReq) *entity.ProvenTxToSync {
		return &entity.ProvenTxToSync{
			TxID:     row.TxID,
			Attempts: row.Attempts,
			Status:   row.Status,
		}
	}), nil
}

func (p *ProvenTxReq) UpdateProvenTxAsMined(ctx context.Context, provenTxAsMined *entity.ProvenTxAsMined) error {
	err := p.db.WithContext(ctx).Model(&models.ProvenTxReq{}).
		Where("tx_id = ?", provenTxAsMined.TxID).
		Updates(&models.ProvenTxReq{
			Status:      wdk.ProvenTxStatusCompleted,
			BlockHash:   &provenTxAsMined.BlockHash,
			BlockHeight: &provenTxAsMined.BlockHeight,
			MerklePath:  provenTxAsMined.MerklePath,
			MerkleRoot:  &provenTxAsMined.MerkleRoot,
			Notified:    true,
		}).Error
	if err != nil {
		return fmt.Errorf("failed to update proven tx as mined: %w", err)
	}
	return nil
}

func (p *ProvenTxReq) IncreaseProvenTxAttemptsForTxIDs(ctx context.Context, txIDs []string) error {
	if len(txIDs) == 0 {
		return nil
	}

	err := p.db.WithContext(ctx).Model(&models.ProvenTxReq{}).
		Where("tx_id IN ? ", txIDs).
		UpdateColumn("attempts", gorm.Expr("attempts + 1")).Error
	if err != nil {
		return fmt.Errorf("failed to increase attempts for tx ids: %w", err)
	}
	return nil
}

func (p *ProvenTxReq) SetStatusForProvenTxAboveAttempts(ctx context.Context, attempts uint64, status wdk.ProvenTxReqStatus) error {
	if attempts == 0 {
		return nil
	}

	err := p.db.WithContext(ctx).Model(&models.ProvenTxReq{}).
		Where("attempts >= ? ", attempts).
		UpdateColumn("status", status).Error
	if err != nil {
		return fmt.Errorf("failed to set status for proven tx above attempts: %w", err)
	}
	return nil
}
