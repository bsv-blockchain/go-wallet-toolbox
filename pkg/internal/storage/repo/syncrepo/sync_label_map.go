package syncrepo

import (
	"context"

	"github.com/go-softwarelab/common/pkg/slices"
	"gorm.io/gorm"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/genquery"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type SyncLabelMap struct {
	common *labelTagMapCommons[models.TxLabelsMap]
	db     *gorm.DB
	query  *genquery.Query
}

func NewSyncLabelMap(db *gorm.DB, query *genquery.Query) *SyncLabelMap {
	return &SyncLabelMap{
		common: &labelTagMapCommons[models.TxLabelsMap]{
			db:                     db,
			query:                  query,
			relationTableName:      query.TxLabelsMap.TableName(),
			parentTableName:        query.TxLabel.TableName(),
			relationPkColumn:       query.TxLabelsMap.TxLabelID.ColumnName().String(),
			parentPkColumn:         query.TxLabel.TxLabelID.ColumnName().String(),
			relationParentIDColumn: query.TxLabelsMap.TransactionID.ColumnName().String(),
		},
		db:    db,
		query: query,
	}
}

func (s *SyncLabelMap) FindLabelsMapForSync(ctx context.Context, userID int, opts ...queryopts.Options) ([]*wdk.TableTxLabelMap, error) {
	result, err := s.common.FindChunk(ctx, userID, opts...)
	if err != nil {
		return nil, err
	}

	return slices.Map(result, s.mapModelToTableTxLabelMap), nil
}

func (s *SyncLabelMap) mapModelToTableTxLabelMap(model *models.TxLabelsMap) *wdk.TableTxLabelMap {
	return &wdk.TableTxLabelMap{
		CreatedAt:     model.CreatedAt,
		UpdatedAt:     model.UpdatedAt,
		TransactionID: uint(model.TransactionID),
		TxLabelID:     uint(model.TxLabelID),
		IsDeleted:     model.IsDeleted,
	}
}

func (s *SyncLabelMap) UpsertLabelMapForSync(ctx context.Context, e *entity.TxLabelsMap) (isNew bool, err error) {
	return s.common.Upsert(ctx, uint(e.TransactionID), uint(e.TxLabelID), e.UpdatedAt)
}

func (s *SyncLabelMap) DeleteLabelMapForSync(ctx context.Context, e *entity.TxLabelsMap) (deleted bool, err error) {
	return s.common.Delete(ctx, uint(e.TransactionID), uint(e.TxLabelID))
}
