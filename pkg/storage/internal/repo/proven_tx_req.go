package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage/internal/database/models"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage/internal/entity"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-sdk/transaction"
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

	model.Status = status
	model.AddNote(time.Now(), historyNote, historyAttrs)

	return db.Where("tx_id", txID).Updates(&model).Error
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

func (p *ProvenTxReq) BuildValidBEEF(ctx context.Context, txID string, sourceTxsStatusFilter []wdk.ProvenTxReqStatus) (*transaction.Beef, error) {
	beef := transaction.NewBeefV2()
	err := p.recursiveBuildValidBEEF(ctx, 0, beef, txID, sourceTxsStatusFilter)
	if err != nil {
		return nil, fmt.Errorf("failed to build valid BEEF: %w", err)
	}

	return beef, nil
}

func (p *ProvenTxReq) recursiveBuildValidBEEF(ctx context.Context, depth int, mergeToBeef *transaction.Beef, txID string, statusFilter []wdk.ProvenTxReqStatus) error {
	if depth > maxDepthOfRecursion {
		return fmt.Errorf("max depth of recursion reached: %d", maxDepthOfRecursion)
	}

	var model models.ProvenTxReq
	query := p.db.WithContext(ctx).
		Model(&model).
		Select("raw_tx, input_beef")

	queryForSubjectTx := depth == 0
	if !queryForSubjectTx && len(statusFilter) > 0 {
		query = query.Where("status IN ? ", statusFilter)
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
			err = p.recursiveBuildValidBEEF(ctx, depth+1, mergeToBeef, sourceTXID, statusFilter)
			if err != nil {
				return fmt.Errorf("failed to recursively find proven tx and merge into BEEF: %w", err)
			}
		}
	}

	// Result is in mergeToBeef
	return nil
}
