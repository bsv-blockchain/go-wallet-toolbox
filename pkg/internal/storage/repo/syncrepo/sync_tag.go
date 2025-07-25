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

type SyncTag struct {
	common *labelTagCommons[models.Tag, models.OutputTag, TagReadModel]
	db     *gorm.DB
	query  *genquery.Query
}

func NewSyncTag(db *gorm.DB, query *genquery.Query) *SyncTag {
	return &SyncTag{
		common: &labelTagCommons[models.Tag, models.OutputTag, TagReadModel]{
			db:                   db,
			query:                query,
			tableName:            query.Tag.TableName(),
			relationUserIDColumn: query.OutputTag.TagUserID.ColumnName().String(),
			relationNameColumn:   query.OutputTag.TagName.ColumnName().String(),
		},
		db:    db,
		query: query,
	}
}

type TagReadModel struct {
	models.Tag
	NumID uint
}

func (s *SyncTag) FindTagsForSync(ctx context.Context, userID int, opts ...queryopts.Options) ([]*wdk.TableOutputTag, error) {
	result, err := s.common.FindChunk(ctx, userID, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to find tags for sync: %w", err)
	}

	return slices.Map(result, s.mapModelToTableTag), nil
}

func (s *SyncTag) UpsertTagForSync(ctx context.Context, entity *entity.Tag) (isNew bool, tagNumID uint, err error) {
	model := models.Tag{
		CreatedAt: entity.CreatedAt,
		UpdatedAt: entity.UpdatedAt,
		UserID:    entity.UserID,
		Name:      entity.Name,
	}

	return s.common.Upsert(ctx, entity.UserID, entity.Name, &model)
}

func (s *SyncTag) DeleteTagForSync(ctx context.Context, entity *entity.Tag) (deleted bool, err error) {
	return s.common.Delete(ctx, entity.UserID, entity.Name)
}

func (s *SyncTag) FindTagByNumIDForSync(ctx context.Context, numID uint) (*entity.Tag, error) {
	model, err := s.common.FindByNumID(ctx, numID)
	if err != nil {
		return nil, err
	}

	if model == nil {
		return nil, nil
	}

	return &entity.Tag{
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.UpdatedAt,
		UserID:    model.UserID,
		Name:      model.Name,
	}, nil
}

func (s *SyncTag) mapModelToTableTag(model *TagReadModel) *wdk.TableOutputTag {
	return &wdk.TableOutputTag{
		CreatedAt:   model.CreatedAt,
		UpdatedAt:   model.UpdatedAt,
		OutputTagID: model.NumID,
		UserID:      model.UserID,
		Tag:         model.Name,
		IsDeleted:   model.DeletedAt.Valid,
	}
}
