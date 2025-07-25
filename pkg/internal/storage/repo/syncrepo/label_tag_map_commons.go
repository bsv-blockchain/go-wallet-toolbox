package syncrepo

import (
	"context"
	"fmt"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/genquery"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/scopes"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/go-softwarelab/common/pkg/to"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type labelTagMapCommons[Model, ReadModel any] struct {
	db                     *gorm.DB
	query                  *genquery.Query
	subjectTableName       string
	relationTableName      string
	relationUserIDColumn   string
	relationNameColumn     string
	relationParentIDColumn string
}

func (f *labelTagMapCommons[_, ReadModel]) FindChunk(ctx context.Context, userID int, opts ...queryopts.Options) ([]*ReadModel, error) {
	labelStringIDClause := fmt.Sprintf("CONCAT(%s, '.', %s)", f.relationUserIDColumn, f.relationNameColumn)
	var resultModels []*ReadModel

	scopesToApply := []func(*gorm.DB) *gorm.DB{
		joinWithNumericIDLookupScope(f.query, labelStringIDClause, f.subjectTableName, clause.InnerJoin),
	}

	options := queryopts.MergeOptions(opts)
	if options.Page != nil {
		scopesToApply = append(scopesToApply, scopes.Paginate(options.Page))
	}

	if options.Since != nil {
		scopesToApply = append(scopesToApply, f.sinceUpdateOrDeleteScope(options.Since.Time))
	}

	err := f.db.WithContext(ctx).
		Model(f.zeroModelPtr()).
		Select(fmt.Sprintf("%s.*, num_id", f.relationTableName)).
		Scopes(scopesToApply...).
		Where(fmt.Sprintf("%s = ?", f.relationUserIDColumn), userID).
		Unscoped().
		Find(&resultModels).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find many2many relation %q: %w", f.relationTableName, err)
	}

	return resultModels, nil
}

func (f *labelTagMapCommons[Model, _]) Upsert(ctx context.Context, parentID uint, userID int, name string, updatedAt time.Time) (isNew bool, err error) {
	err = f.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updateTx := tx.Model(f.zeroModelPtr()).
			Where(fmt.Sprintf("%s = ?", f.relationParentIDColumn), parentID).
			Where(fmt.Sprintf("%s = ?", f.relationUserIDColumn), userID).
			Where(fmt.Sprintf("%s = ?", f.relationNameColumn), name).
			UpdateColumn("updated_at", updatedAt)

		if updateTx.Error != nil {
			return fmt.Errorf("failed to update many2many relation: %w", updateTx.Error)
		}

		if updateTx.RowsAffected > 0 {
			return nil
		}

		err = tx.Model(f.zeroModelPtr()).Create(map[string]any{
			f.relationParentIDColumn: parentID,
			f.relationUserIDColumn:   userID,
			f.relationNameColumn:     name,
			"updated_at":             updatedAt,
		}).Error
		if err != nil {
			return fmt.Errorf("failed to create many2many relation: %w", err)
		}

		isNew = true

		return nil
	})

	if err != nil {
		return false, fmt.Errorf("transaction failed for %s: %w", f.relationTableName, err)
	}

	return isNew, nil
}

func (f *labelTagMapCommons[_, _]) Delete(ctx context.Context, parentID uint, userID int, name string) (deleted bool, err error) {
	txDelete := f.db.WithContext(ctx).Delete(
		f.zeroModelPtr(),
		fmt.Sprintf("%s = ? AND %s = ? AND %s = ?", f.relationParentIDColumn, f.relationUserIDColumn, f.relationNameColumn),
		parentID, userID, name,
	)
	if txDelete.Error != nil {
		return false, fmt.Errorf("failed to delete many2many relation %q: %w", f.relationTableName, txDelete.Error)
	}

	deleted = txDelete.RowsAffected > 0
	return deleted, nil
}

func (f *labelTagMapCommons[Model, _]) zeroModelPtr() *Model {
	return to.Ptr(to.ZeroValue[Model]())
}

func (f *labelTagMapCommons[_, _]) sinceUpdateOrDeleteScope(since time.Time) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("updated_at >= ? OR deleted_at >= ?", since, since)
	}
}
