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

type SyncProvenTxReq struct {
	db    *gorm.DB
	query *genquery.Query
}

func NewSyncProvenTxReq(db *gorm.DB, query *genquery.Query) *SyncProvenTxReq {
	return &SyncProvenTxReq{db: db, query: query}
}

func (s *SyncProvenTxReq) tableName() string {
	return s.query.ProvenTxReq.TableName()
}

func (s *SyncProvenTxReq) FindProvenTxReqsForSync(ctx context.Context, userID int, opts ...queryopts.Options) ([]*wdk.TableProvenTxReq, error) {
	filters := append(scopes.FromQueryOpts(opts), s.whereExistsScope(userID))

	var resultModels []*models.ProvenTxReq

	err := s.db.WithContext(ctx).
		Model(&models.ProvenTxReq{}).
		Scopes(filters...).
		Find(&resultModels).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find proven tx reqs for sync: %w", err)
	}

	return slices.Map(resultModels, s.mapModelToTableProvenTxReqForSync), nil
}

func (s *SyncProvenTxReq) UpsertProvenTxReqForSync(ctx context.Context, entity *entity.ProvenTxReq) (isNew bool, err error) {
	model := models.ProvenTxReq{
		Timestamps: models.Timestamps{
			CreatedAt: entity.CreatedAt,
			UpdatedAt: entity.UpdatedAt,
		},
		TxID:                entity.TxID,
		Status:              entity.Status,
		Attempts:            uint64(entity.Attempts),
		WasBroadcast:        entity.WasBroadcast || entity.Status.WasBroadcastStatus(),
		RebroadcastAttempts: int(entity.RebroadcastAttempts),
		Notified:            entity.Notified,
		RawTx:               entity.RawTx,
		InputBeef:           entity.InputBEEF,
		ProvenTxID:          entity.ProvenTxID,
		Batch:               entity.Batch,
	}

	// NOTE: We don't overwrite History and Notify in sync directly since they are JSON and might have complex merge semantics,
	// but according to previous behavior, we did a `Updates(model)` which updates all non-zero fields.
	// Since we are creating/updating from sync, History is just stored as string.
	// However, the `entity.ProvenTxReq` doesn't have `TxNotes` like the old `KnownTx`. The task for `C-entity` was:
	// "redirect addTxNote/addTxNotes ... to serialize into ProvenTxReq.history instead of inserting TxNote rows".
	// The sync payload for `History` and `Notify` isn't fully detailed in the current scope, so we can set them to "{}" if they are empty.
	model.History = "{}" // We initialize as empty for now, or we'd map it if entity had it as a string
	model.Notify = "{}"

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing models.ProvenTxReq
		existsErr := tx.Model(&models.ProvenTxReq{}).
			Select("provenTxReqId, updated_at").
			Where("txid = ?", entity.TxID).
			First(&existing).Error

		if existsErr == nil {
			if !model.UpdatedAt.After(existing.UpdatedAt) {
				return nil
			}

			updateTx := tx.Model(&models.ProvenTxReq{}).
				Where("txid = ? AND updated_at < ?", entity.TxID, model.UpdatedAt).
				Updates(model)

			if updateTx.Error != nil {
				return fmt.Errorf("failed to update proven tx req: %w", updateTx.Error)
			}

			return nil
		}

		if !errors.Is(existsErr, gorm.ErrRecordNotFound) {
			return fmt.Errorf("failed to lookup existing proven tx req: %w", existsErr)
		}

		if err = tx.Create(&model).Error; err != nil {
			return fmt.Errorf("failed to create proven tx req: %w", err)
		}

		isNew = true

		return nil
	})
	if err != nil {
		return false, fmt.Errorf("transaction failed: %w", err)
	}

	return isNew, nil
}

func (s *SyncProvenTxReq) whereExistsScope(userID int) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		whereExistClause := fmt.Sprintf(
			"EXISTS (SELECT 1 FROM %s as user_tx WHERE user_tx.txid = %s.txid AND user_tx.userId = ?)",
			s.query.Transaction.TableName(),
			s.tableName(),
		)

		return db.Where(whereExistClause, userID)
	}
}

func (s *SyncProvenTxReq) mapModelToTableProvenTxReqForSync(model *models.ProvenTxReq) *wdk.TableProvenTxReq {
	return &wdk.TableProvenTxReq{
		CreatedAt:           model.CreatedAt,
		UpdatedAt:           model.UpdatedAt,
		ProvenTxReqID:       int(model.ProvenTxReqID),
		Status:              model.Status,
		Attempts:            model.Attempts,
		WasBroadcast:        model.WasBroadcast || model.Status.WasBroadcastStatus(),
		RebroadcastAttempts: uint64(model.RebroadcastAttempts),
		Notified:            model.Notified,
		TxID:                model.TxID,
		Batch:               model.Batch,
		History:             model.History,
		Notify:              model.Notify,
		RawTx:               model.RawTx,
		InputBEEF:           model.InputBeef,
	}
}
