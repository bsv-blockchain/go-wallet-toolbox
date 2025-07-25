package syncrepo

import (
	"context"
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/genquery"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/slices"
	"gorm.io/gorm"
)

type SyncLabel struct {
	common *labelTagCommons[models.Label, models.TransactionLabel, LabelReadModel]
	db     *gorm.DB
	query  *genquery.Query
}

func NewSyncLabel(db *gorm.DB, query *genquery.Query) *SyncLabel {
	return &SyncLabel{
		common: &labelTagCommons[models.Label, models.TransactionLabel, LabelReadModel]{
			db:                   db,
			query:                query,
			tableName:            query.Label.TableName(),
			relationUserIDColumn: query.TransactionLabel.LabelUserID.ColumnName().String(),
			relationNameColumn:   query.TransactionLabel.LabelName.ColumnName().String(),
		},
		db:    db,
		query: query,
	}
}

type LabelReadModel struct {
	models.Label
	NumID uint
}

func (s *SyncLabel) FindLabelsForSync(ctx context.Context, userID int, opts ...queryopts.Options) ([]*wdk.TableTxLabel, error) {
	result, err := s.common.FindChunk(ctx, userID, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to find tags for sync: %w", err)
	}

	return slices.Map(result, s.mapModelToTableTxLabel), nil
}

func (s *SyncLabel) UpsertLabelForSync(ctx context.Context, entity *entity.Label) (isNew bool, labelNumID uint, err error) {
	model := models.Label{
		CreatedAt: entity.CreatedAt,
		UpdatedAt: entity.UpdatedAt,
		UserID:    entity.UserID,
		Name:      entity.Name,
	}

	return s.common.Upsert(ctx, entity.UserID, entity.Name, &model)
}

func (s *SyncLabel) DeleteLabelForSync(ctx context.Context, entity *entity.Label) (deleted bool, err error) {
	return s.common.Delete(ctx, entity.UserID, entity.Name)
}

func (s *SyncLabel) FindLabelByNumIDForSync(ctx context.Context, numID uint) (*entity.Label, error) {
	model, err := s.common.FindByNumID(ctx, numID)
	if err != nil {
		return nil, err
	}

	if model == nil {
		return nil, nil
	}

	return &entity.Label{
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.UpdatedAt,
		UserID:    model.UserID,
		Name:      model.Name,
	}, nil
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
