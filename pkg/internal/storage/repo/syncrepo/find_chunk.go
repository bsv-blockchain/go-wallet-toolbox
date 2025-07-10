package syncrepo

import (
	"context"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type gormScope = func(*gorm.DB) *gorm.DB

func findChunk[Model, ResultModel any](
	ctx context.Context,
	db *gorm.DB,
	tableName, stringID string,
	scopes []gormScope,
) ([]*ResultModel, error) {
	var resultModels []*ResultModel

	var model Model

	err := db.Transaction(func(tx *gorm.DB) error {
		if err := upsertNumericIDLookup(ctx, db, tx, func(db *gorm.DB) *gorm.DB {
			return db.
				Select(fmt.Sprintf("?, %s", stringID), tableName).
				Scopes(scopes...).
				Find(&model)
		}); err != nil {
			return fmt.Errorf("failed to upsert numeric ID lookup: %w", err)
		}

		if err := tx.WithContext(ctx).
			Model(&model).
			Select("*").
			Scopes(joinWithNumericIDLookupScope(stringID, tableName, clause.InnerJoin)).
			Scopes(scopes...).
			Find(&resultModels).Error; err != nil {
			return fmt.Errorf("failed to find proven tx requests for sync: %w", err)
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("transaction failed: %w", err)
	}

	return resultModels, nil
}
