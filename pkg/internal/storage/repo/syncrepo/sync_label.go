package syncrepo

import (
	"context"
	"fmt"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/genquery"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/scopes"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/slices"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SyncLabel struct {
	db *gorm.DB
}

func NewSyncLabel(db *gorm.DB) *SyncLabel {
	return &SyncLabel{db: db}
}

type LabelReadModel struct {
	models.Label
	NumID uint
}

func (s *SyncLabel) tableName() string {
	return genquery.Label.TableName()
}

func (s *SyncLabel) FindLabelsForSync(ctx context.Context, userID int, opts ...queryopts.Options) ([]*wdk.TableTxLabel, error) {
	const labelStringIDClause = "CONCAT(user_id, '.', name)"
	var resultModels []*LabelReadModel

	err := s.db.Transaction(func(tx *gorm.DB) error {
		filters := append(scopes.FromQueryOpts(opts), scopes.UserID(userID))

		err := upsertNumericIDLookup(ctx, s.db, tx, func(db *gorm.DB) *gorm.DB {
			return db.
				Select(fmt.Sprintf("?, %s", labelStringIDClause), s.tableName()).
				Scopes(filters...).
				Unscoped().
				Find(&models.Label{})
		})
		if err != nil {
			return err
		}

		err = tx.WithContext(ctx).
			Model(&models.Label{}).
			Select("*").
			Scopes(filters...).
			Scopes(joinWithNumericIDLookupScope(labelStringIDClause, s.tableName(), clause.InnerJoin)).
			Unscoped().
			Find(&resultModels).Error
		if err != nil {
			return fmt.Errorf("failed to find labels for sync: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("transaction failed: %w", err)
	}

	return slices.Map(resultModels, s.mapModelToTableTxLabel), nil
}

func (s *SyncLabel) UpsertLabelForSync(ctx context.Context, entity *entity.Label) (isNew bool, labelNumID uint, err error) {
	model := models.Label{
		CreatedAt: entity.CreatedAt,
		UpdatedAt: entity.UpdatedAt,
		UserID:    entity.UserID,
		Name:      entity.Name,
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		numID, err := s.saveNumericIDForLabel(ctx, tx, entity.UserID, entity.Name)
		if err != nil {
			return err
		}

		labelNumID = numID

		updateTx := tx.Model(&models.Label{}).
			Where("user_id = ? AND name = ?", entity.UserID, model.Name).
			Updates(model)

		if updateTx.Error != nil {
			return fmt.Errorf("failed to update label: %w", updateTx.Error)
		}

		if updateTx.RowsAffected > 0 {
			return nil
		}

		err = tx.Create(&model).Error
		if err != nil {
			return fmt.Errorf("failed to create label: %w", err)
		}

		isNew = true

		return nil
	})

	if err != nil {
		return false, 0, fmt.Errorf("transaction failed: %w", err)
	}

	return isNew, labelNumID, nil
}

func (s *SyncLabel) DeleteLabelForSync(ctx context.Context, entity *entity.Label) (deleted bool, err error) {
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txDelete := tx.Delete(
			&models.Label{},
			"user_id = ? AND name = ?", entity.UserID, entity.Name,
		)
		if txDelete.Error != nil {
			return fmt.Errorf("failed to delete label: %w", err)
		}

		deleted = txDelete.RowsAffected > 0

		err = tx.Delete(
			&models.TransactionLabel{},
			"label_user_id = ? AND label_name = ?", entity.UserID, entity.Name,
		).Error
		if err != nil {
			return fmt.Errorf("failed to delete label map entries: %w", err)
		}

		return nil
	})

	if err != nil {
		return false, fmt.Errorf("transaction failed: %w", err)
	}

	return deleted, nil
}

func (s *SyncLabel) mapModelToTableTxLabel(model *LabelReadModel) *wdk.TableTxLabel {
	return &wdk.TableTxLabel{
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.UpdatedAt,
		TxLabelID: model.NumID,
		UserID:    model.UserID,
		Label:     model.Name,
		IsDeleted: model.DeletedAt.Valid,
	}
}
func (s *SyncLabel) saveNumericIDForLabel(ctx context.Context, tx *gorm.DB, userID int, labelName string) (uint, error) {
	stringID := fmt.Sprintf("%d.%s", userID, labelName)

	err := saveNumericIDLookup(ctx, tx, s.tableName(), stringID)
	if err != nil {
		return 0, fmt.Errorf("failed to save numeric ID lookup for label: %w", err)
	}

	return findNumericIDLookup(ctx, tx, s.tableName(), stringID)
}
