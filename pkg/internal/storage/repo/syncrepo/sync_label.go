package syncrepo

import (
	"context"
	"fmt"

	"github.com/go-softwarelab/common/pkg/slices"
	"gorm.io/gorm"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/genquery"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type SyncLabel struct {
	common *labelTagCommons[models.TxLabel, models.TxLabelsMap, models.TxLabel]
	db     *gorm.DB
	query  *genquery.Query
}

func NewSyncLabel(db *gorm.DB, query *genquery.Query) *SyncLabel {
	return &SyncLabel{
		common: &labelTagCommons[models.TxLabel, models.TxLabelsMap, models.TxLabel]{
			db:                   db,
			query:                query,
			tableName:            query.TxLabel.TableName(),
			relationUserIDColumn: query.TxLabel.UserID.ColumnName().String(),
			relationValueColumn:  query.TxLabel.Label.ColumnName().String(),
		},
		db:    db,
		query: query,
	}
}

func (s *SyncLabel) FindLabelsForSync(ctx context.Context, userID int, opts ...queryopts.Options) ([]*wdk.TableTxLabel, error) {
	result, err := s.common.FindChunk(ctx, userID, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to find labels for sync: %w", err)
	}

	return slices.Map(result, s.mapModelToTableTxLabel), nil
}

func (s *SyncLabel) UpsertLabelForSync(ctx context.Context, entity *entity.TxLabel) (isNew bool, labelID uint, err error) {
	model := models.TxLabel{
		Timestamps: models.Timestamps{
			CreatedAt: entity.CreatedAt,
			UpdatedAt: entity.UpdatedAt,
		},
		UserID:    entity.UserID,
		Label:     entity.Label,
		IsDeleted: false,
	}

	isNew, _, err = s.common.Upsert(ctx, entity.UserID, entity.Label, &model)
	return isNew, model.TxLabelID, err
}

func (s *SyncLabel) DeleteLabelForSync(ctx context.Context, entity *entity.TxLabel) (deleted bool, err error) {
	return s.common.Delete(ctx, entity.UserID, entity.Label)
}

func (s *SyncLabel) FindLabelByIDForSync(ctx context.Context, labelID uint) (*entity.TxLabel, error) {
	model, err := s.common.FindByID(ctx, labelID, "txLabelId")
	if err != nil {
		return nil, err
	}

	if model == nil {
		return nil, nil
	}

	return &entity.TxLabel{
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.UpdatedAt,
		UserID:    model.UserID,
		Label:     model.Label,
	}, nil
}

func (s *SyncLabel) mapModelToTableTxLabel(model *models.TxLabel) *wdk.TableTxLabel {
	return &wdk.TableTxLabel{
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.UpdatedAt,
		TxLabelID: uint(model.TxLabelID),
		UserID:    model.UserID,
		Label:     model.Label,
		IsDeleted: model.IsDeleted,
	}
}
