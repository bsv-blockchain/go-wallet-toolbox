package syncrepo

import (
	"context"
	"errors"
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

type SyncLabelMap struct {
	db *gorm.DB
}

func NewSyncLabelMap(db *gorm.DB) *SyncLabelMap {
	return &SyncLabelMap{db: db}
}

type LabelsMapReadModel struct {
	models.TransactionLabel
	NumID uint
}

func (s *SyncLabelMap) tableName() string {
	return genquery.TransactionLabel.TableName()
}

func (s *SyncLabelMap) FindLabelsMapForSync(ctx context.Context, userID int, opts ...queryopts.Options) ([]*wdk.TableTxLabelMap, error) {
	const labelStringIDClause = "CONCAT(label_user_id, '.', label_name)"
	var resultModels []*LabelsMapReadModel

	err := s.db.WithContext(ctx).
		Model(models.TransactionLabel{}).
		Select(fmt.Sprintf("%s.*, num_id", s.tableName())).
		Scopes(scopes.FromQueryOpts(opts)...).
		Scopes(joinWithNumericIDLookupScope(labelStringIDClause, genquery.Label.TableName(), clause.InnerJoin)).
		Where("label_user_id = ?", userID).
		Unscoped().
		Find(&resultModels).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find labels map for sync: %w", err)
	}

	return slices.Map(resultModels, s.mapModelToTableTxLabelMap), nil
}

func (s *SyncLabelMap) mapModelToTableTxLabelMap(model *LabelsMapReadModel) *wdk.TableTxLabelMap {
	return &wdk.TableTxLabelMap{
		CreatedAt:     model.CreatedAt,
		UpdatedAt:     model.UpdatedAt,
		TransactionID: model.TransactionID,
		TxLabelID:     model.NumID,
		IsDeleted:     model.DeletedAt.Valid,
	}
}

func (s *SyncLabelMap) FindLabelByNumIDForSync(ctx context.Context, labelNumID uint) (*entity.Label, error) {
	const labelStringIDClause = "CONCAT(user_id, '.', name)"
	var label models.Label

	err := s.db.WithContext(ctx).Model(&models.Label{}).
		Scopes(joinWithNumericIDLookupScope(labelStringIDClause, genquery.Label.TableName(), clause.InnerJoin)).
		Where("num.num_id = ?", labelNumID).
		First(&label).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find label name by numeric ID: %w", err)
	}

	return &entity.Label{
		CreatedAt: label.CreatedAt,
		UpdatedAt: label.UpdatedAt,
		UserID:    label.UserID,
		Name:      label.Name,
	}, nil
}

func (s *SyncLabelMap) UpsertLabelMapForSync(ctx context.Context, entity *entity.LabelMap) (isNew bool, err error) {
	model := models.TransactionLabel{
		CreatedAt:     entity.CreatedAt,
		UpdatedAt:     entity.UpdatedAt,
		TransactionID: entity.TransactionID,
		LabelUserID:   entity.UserID,
		LabelName:     entity.Name,
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updateTx := tx.Model(&models.TransactionLabel{}).
			Where("transaction_id = ? AND label_user_id = ? AND label_name = ?", model.TransactionID, model.LabelUserID, model.LabelName).
			UpdateColumn("updated_at", model.UpdatedAt)

		if updateTx.Error != nil {
			return fmt.Errorf("failed to update label map: %w", updateTx.Error)
		}

		if updateTx.RowsAffected > 0 {
			return nil
		}

		err := tx.Create(&model).Error
		if err != nil {
			return fmt.Errorf("failed to create label map: %w", err)
		}

		isNew = true

		return nil
	})

	if err != nil {
		return false, fmt.Errorf("transaction failed: %w", err)
	}

	return isNew, nil
}

func (s *SyncLabelMap) DeleteLabelMapForSync(ctx context.Context, entity *entity.LabelMap) (deleted bool, err error) {
	txDelete := s.db.WithContext(ctx).Delete(
		&models.TransactionLabel{},
		"transaction_id = ? AND label_user_id = ? AND label_name = ?", entity.TransactionID, entity.UserID, entity.Name,
	)
	if txDelete.Error != nil {
		return false, fmt.Errorf("failed to delete label: %w", err)
	}

	deleted = txDelete.RowsAffected > 0
	return deleted, nil
}
