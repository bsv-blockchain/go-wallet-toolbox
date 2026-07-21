package syncrepo

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-softwarelab/common/pkg/slices"
	"gorm.io/gorm"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/genquery"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/scopes"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type SyncProvenTx struct {
	db    *gorm.DB
	query *genquery.Query
}

func NewSyncProvenTx(db *gorm.DB, query *genquery.Query) *SyncProvenTx {
	return &SyncProvenTx{db: db, query: query}
}

func (s *SyncProvenTx) tableName() string {
	return s.query.ProvenTx.TableName()
}

func (s *SyncProvenTx) FindProvenTxsForSync(ctx context.Context, userID int, opts ...queryopts.Options) ([]*wdk.TableProvenTx, error) {
	filters := append(scopes.FromQueryOpts(opts), s.whereExistsScope(userID))

	var resultModels []*models.ProvenTx

	err := s.db.WithContext(ctx).
		Model(&models.ProvenTx{}).
		Scopes(filters...).
		Find(&resultModels).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find proven txs for sync: %w", err)
	}

	return slices.Map(resultModels, s.mapModelToTableProvenTxForSync), nil
}

func (s *SyncProvenTx) UpsertProvenTxForSync(ctx context.Context, entity *entity.ProvenTx) (isNew bool, provenTxID uint, err error) {
	model := models.ProvenTx{
		Timestamps: models.Timestamps{
			CreatedAt: entity.CreatedAt,
			UpdatedAt: entity.UpdatedAt,
		},
		TxID:       entity.TxID,
		Height:     &entity.Height,
		Index:      func() *uint64 { v := uint64(entity.Index); return &v }(),
		MerklePath: entity.MerklePath,
		RawTx:      entity.RawTx,
		BlockHash:  &entity.BlockHash,
		MerkleRoot: &entity.MerkleRoot,
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing models.ProvenTx
		existsErr := tx.Model(&models.ProvenTx{}).
			Select("provenTxId, updated_at").
			Where("txid = ?", entity.TxID).
			First(&existing).Error

		if existsErr == nil {
			provenTxID = existing.ProvenTxID

			if !model.UpdatedAt.After(existing.UpdatedAt) {
				return nil
			}

			updateTx := tx.Model(&models.ProvenTx{}).
				Where("txid = ? AND updated_at < ?", entity.TxID, model.UpdatedAt).
				Updates(model)

			if updateTx.Error != nil {
				return fmt.Errorf("failed to update proven tx: %w", updateTx.Error)
			}

			return nil
		}

		if !errors.Is(existsErr, gorm.ErrRecordNotFound) {
			return fmt.Errorf("failed to lookup existing proven tx: %w", existsErr)
		}

		if err = tx.Create(&model).Error; err != nil {
			return fmt.Errorf("failed to create proven tx: %w", err)
		}

		provenTxID = model.ProvenTxID
		isNew = true

		return nil
	})
	if err != nil {
		return false, 0, fmt.Errorf("transaction failed: %w", err)
	}

	return isNew, provenTxID, nil
}

func (s *SyncProvenTx) whereExistsScope(userID int) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		whereExistClause := fmt.Sprintf(
			"EXISTS (SELECT 1 FROM %s as user_tx WHERE user_tx.txid = %s.txid AND user_tx.userId = ?)",
			s.query.Transaction.TableName(),
			s.tableName(),
		)

		return db.Where(whereExistClause, userID)
	}
}

func (s *SyncProvenTx) mapModelToTableProvenTxForSync(model *models.ProvenTx) *wdk.TableProvenTx {
	var height uint32
	if model.Height != nil {
		height = *model.Height
	}
	var index uint64
	if model.Index != nil {
		index = *model.Index
	}
	var blockHash string
	if model.BlockHash != nil {
		blockHash = *model.BlockHash
	}
	var merkleRoot string
	if model.MerkleRoot != nil {
		merkleRoot = *model.MerkleRoot
	}

	return &wdk.TableProvenTx{
		CreatedAt:  model.CreatedAt,
		UpdatedAt:  model.UpdatedAt,
		ProvenTxID: int(model.ProvenTxID),
		TxID:       model.TxID,
		Height:     height,
		Index:      int(index),
		MerklePath: model.MerklePath,
		RawTx:      model.RawTx,
		BlockHash:  blockHash,
		MerkleRoot: merkleRoot,
	}
}
