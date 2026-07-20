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

type SyncTag struct {
	common *labelTagCommons[models.OutputTag, models.OutputTagsMap, models.OutputTag]
	db     *gorm.DB
	query  *genquery.Query
}

func NewSyncTag(db *gorm.DB, query *genquery.Query) *SyncTag {
	return &SyncTag{
		common: &labelTagCommons[models.OutputTag, models.OutputTagsMap, models.OutputTag]{
			db:                   db,
			query:                query,
			tableName:            query.OutputTag.TableName(),
			relationUserIDColumn: query.OutputTag.UserID.ColumnName().String(),
			relationValueColumn:  query.OutputTag.Tag.ColumnName().String(),
		},
		db:    db,
		query: query,
	}
}

func (s *SyncTag) FindTagsForSync(ctx context.Context, userID int, opts ...queryopts.Options) ([]*wdk.TableOutputTag, error) {
	result, err := s.common.FindChunk(ctx, userID, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to find tags for sync: %w", err)
	}

	return slices.Map(result, s.mapModelToTableTag), nil
}

func (s *SyncTag) UpsertTagForSync(ctx context.Context, entity *entity.OutputTag) (isNew bool, tagID uint, err error) {
	model := models.OutputTag{
		Timestamps: models.Timestamps{
			CreatedAt: entity.CreatedAt,
			UpdatedAt: entity.UpdatedAt,
		},
		UserID:    entity.UserID,
		Tag:       entity.Tag,
		IsDeleted: false,
	}

	isNew, _, err = s.common.Upsert(ctx, entity.UserID, entity.Tag, &model)
	return isNew, model.OutputTagID, err
}

func (s *SyncTag) DeleteTagForSync(ctx context.Context, entity *entity.OutputTag) (deleted bool, err error) {
	return s.common.Delete(ctx, entity.UserID, entity.Tag)
}

func (s *SyncTag) FindTagByIDForSync(ctx context.Context, tagID uint) (*entity.OutputTag, error) {
	model, err := s.common.FindByID(ctx, tagID, "outputTagId")
	if err != nil {
		return nil, err
	}

	if model == nil {
		return nil, nil
	}

	return &entity.OutputTag{
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.UpdatedAt,
		UserID:    model.UserID,
		Tag:       model.Tag,
	}, nil
}

func (s *SyncTag) mapModelToTableTag(model *models.OutputTag) *wdk.TableOutputTag {
	return &wdk.TableOutputTag{
		CreatedAt:   model.CreatedAt,
		UpdatedAt:   model.UpdatedAt,
		OutputTagID: uint(model.OutputTagID),
		UserID:      model.UserID,
		Tag:         model.Tag,
		IsDeleted:   model.IsDeleted,
	}
}
