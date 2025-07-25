package syncrepo

import (
	"context"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/genquery"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/slices"
	"github.com/go-softwarelab/common/pkg/to"
	"gorm.io/gorm"
)

type SyncLabelMap struct {
	common *labelTagMapCommons[models.TransactionLabel, LabelsMapReadModel]
	db     *gorm.DB
	query  *genquery.Query
}

func NewSyncLabelMap(db *gorm.DB, query *genquery.Query) *SyncLabelMap {
	return &SyncLabelMap{
		common: &labelTagMapCommons[models.TransactionLabel, LabelsMapReadModel]{
			db:                     db,
			query:                  query,
			subjectTableName:       query.Label.TableName(),
			relationTableName:      query.TransactionLabel.TableName(),
			relationUserIDColumn:   query.TransactionLabel.LabelUserID.ColumnName().String(),
			relationNameColumn:     query.TransactionLabel.LabelName.ColumnName().String(),
			relationParentIDColumn: query.TransactionLabel.TransactionID.ColumnName().String(),
		},
		db:    db,
		query: query,
	}
}

type LabelsMapReadModel struct {
	models.TransactionLabel
	NumID uint
}

func (s *SyncLabelMap) FindLabelsMapForSync(ctx context.Context, userID int, opts ...queryopts.Options) ([]*wdk.TableTxLabelMap, error) {
	result, err := s.common.FindChunk(ctx, userID, opts...)
	if err != nil {
		return nil, err
	}

	return slices.Map(result, s.mapModelToTableTxLabelMap), nil
}

func (s *SyncLabelMap) mapModelToTableTxLabelMap(model *LabelsMapReadModel) *wdk.TableTxLabelMap {
	deleted := model.DeletedAt.Valid

	return &wdk.TableTxLabelMap{
		CreatedAt:     model.CreatedAt,
		UpdatedAt:     to.IfThen(!deleted, model.UpdatedAt).ElseThen(model.DeletedAt.Time),
		TransactionID: model.TransactionID,
		TxLabelID:     model.NumID,
		IsDeleted:     deleted,
	}
}

func (s *SyncLabelMap) UpsertLabelMapForSync(ctx context.Context, entity *entity.LabelMap) (isNew bool, err error) {
	return s.common.Upsert(ctx, entity.TransactionID, entity.UserID, entity.Name, entity.UpdatedAt)
}

func (s *SyncLabelMap) DeleteLabelMapForSync(ctx context.Context, entity *entity.LabelMap) (deleted bool, err error) {
	return s.common.Delete(ctx, entity.TransactionID, entity.UserID, entity.Name)
}
