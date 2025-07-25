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

type SyncTagMap struct {
	common *labelTagMapCommons[models.OutputTag, TagsMapReadModel]
	db     *gorm.DB
	query  *genquery.Query
}

func NewSyncTagMap(db *gorm.DB, query *genquery.Query) *SyncTagMap {
	return &SyncTagMap{
		common: &labelTagMapCommons[models.OutputTag, TagsMapReadModel]{
			db:                     db,
			query:                  query,
			subjectTableName:       query.Tag.TableName(),
			relationTableName:      query.OutputTag.TableName(),
			relationUserIDColumn:   query.OutputTag.TagUserID.ColumnName().String(),
			relationNameColumn:     query.OutputTag.TagName.ColumnName().String(),
			relationParentIDColumn: query.OutputTag.OutputID.ColumnName().String(),
		},
		db:    db,
		query: query,
	}
}

type TagsMapReadModel struct {
	models.OutputTag
	NumID uint
}

func (s *SyncTagMap) FindTagsMapForSync(ctx context.Context, userID int, opts ...queryopts.Options) ([]*wdk.TableOutputTagMap, error) {
	result, err := s.common.FindChunk(ctx, userID, opts...)
	if err != nil {
		return nil, err
	}

	return slices.Map(result, s.mapModelToTableOutputTagMap), nil
}

func (s *SyncTagMap) mapModelToTableOutputTagMap(model *TagsMapReadModel) *wdk.TableOutputTagMap {
	deleted := model.DeletedAt.Valid

	return &wdk.TableOutputTagMap{
		CreatedAt:   model.CreatedAt,
		UpdatedAt:   to.IfThen(!deleted, model.UpdatedAt).ElseThen(model.DeletedAt.Time),
		OutputID:    model.OutputID,
		OutputTagID: model.NumID,
		IsDeleted:   deleted,
	}
}

func (s *SyncTagMap) UpsertTagMapForSync(ctx context.Context, entity *entity.TagMap) (isNew bool, err error) {
	return s.common.Upsert(ctx, entity.OutputID, entity.UserID, entity.Name, entity.UpdatedAt)
}

func (s *SyncTagMap) DeleteTagMapForSync(ctx context.Context, entity *entity.TagMap) (deleted bool, err error) {
	return s.common.Delete(ctx, entity.OutputID, entity.UserID, entity.Name)
}
