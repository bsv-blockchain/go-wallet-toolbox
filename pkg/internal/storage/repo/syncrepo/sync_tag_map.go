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

type SyncTagMap struct {
	common *labelTagMapCommons[models.OutputTagsMap]
	db     *gorm.DB
	query  *genquery.Query
}

func NewSyncTagMap(db *gorm.DB, query *genquery.Query) *SyncTagMap {
	return &SyncTagMap{
		common: &labelTagMapCommons[models.OutputTagsMap]{
			db:                     db,
			query:                  query,
			relationTableName:      query.OutputTagsMap.TableName(),
			parentTableName:        query.OutputTag.TableName(),
			relationPkColumn:       query.OutputTagsMap.OutputTagID.ColumnName().String(),
			parentPkColumn:         query.OutputTag.OutputTagID.ColumnName().String(),
			relationParentIDColumn: query.OutputTagsMap.OutputID.ColumnName().String(),
		},
		db:    db,
		query: query,
	}
}

func (s *SyncTagMap) FindTagsMapForSync(ctx context.Context, userID int, opts ...queryopts.Options) ([]*wdk.TableOutputTagMap, error) {
	result, err := s.common.FindChunk(ctx, userID, opts...)
	if err != nil {
		return nil, err
	}

	return slices.Map(result, s.mapModelToTableOutputTagMap), nil
}

func (s *SyncTagMap) mapModelToTableOutputTagMap(model *models.OutputTagsMap) *wdk.TableOutputTagMap {
	return &wdk.TableOutputTagMap{
		CreatedAt:   model.CreatedAt,
		UpdatedAt:   model.UpdatedAt,
		OutputID:    uint(model.OutputID),
		OutputTagID: uint(model.OutputTagID),
		IsDeleted:   model.IsDeleted,
	}
}

func (s *SyncTagMap) UpsertTagMapForSync(ctx context.Context, e *entity.OutputTagsMap) (isNew bool, err error) {
	return s.common.Upsert(ctx, uint(e.OutputID), uint(e.OutputTagID), e.UpdatedAt)
}

func (s *SyncTagMap) DeleteTagMapForSync(ctx context.Context, e *entity.OutputTagsMap) (deleted bool, err error) {
	return s.common.Delete(ctx, uint(e.OutputID), uint(e.OutputTagID))
}
