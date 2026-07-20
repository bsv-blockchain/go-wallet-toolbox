package syncrepo

import (
	"context"
	"fmt"
	"time"

	"github.com/go-softwarelab/common/pkg/to"
	"gorm.io/gorm"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/genquery"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/scopes"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
)

type labelTagMapCommons[Model any] struct {
	db                     *gorm.DB
	query                  *genquery.Query
	relationTableName      string
	parentTableName        string
	relationPkColumn       string // e.g. txLabelId or outputTagId
	parentPkColumn         string // e.g. txLabelId or outputTagId (in parent table)
	relationParentIDColumn string // e.g. transactionId or outputId
}

func (f *labelTagMapCommons[Model]) FindChunk(ctx context.Context, userID int, opts ...queryopts.Options) ([]*Model, error) {
	var resultModels []*Model

	scopesToApply := []func(*gorm.DB) *gorm.DB{
		func(db *gorm.DB) *gorm.DB {
			return db.Joins(fmt.Sprintf("INNER JOIN %s ON %s.%s = %s.%s", f.parentTableName, f.relationTableName, f.relationPkColumn, f.parentTableName, f.parentPkColumn))
		},
		func(db *gorm.DB) *gorm.DB {
			return db.Where(fmt.Sprintf("%s.userId = ?", f.parentTableName), userID)
		},
	}

	options := queryopts.MergeOptions(opts)
	if options.Page != nil {
		scopesToApply = append(scopesToApply, scopes.Paginate(options.Page))
	}

	if options.Since != nil {
		scopesToApply = append(scopesToApply, f.sinceUpdateScope(options.Since.Time))
	}

	err := f.db.WithContext(ctx).
		Model(f.zeroModelPtr()).
		Select(fmt.Sprintf("%s.*", f.relationTableName)).
		Scopes(scopesToApply...).
		Find(&resultModels).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find many2many relation %q: %w", f.relationTableName, err)
	}

	return resultModels, nil
}

func (f *labelTagMapCommons[Model]) Upsert(ctx context.Context, parentID uint, pkID uint, updatedAt time.Time) (isNew bool, err error) {
	err = f.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updateTx := tx.Model(f.zeroModelPtr()).
			Where(fmt.Sprintf("%s = ? AND %s = ?", f.relationParentIDColumn, f.relationPkColumn), parentID, pkID).
			UpdateColumn("updated_at", updatedAt)

		if updateTx.Error != nil {
			return fmt.Errorf("failed to update many2many relation: %w", updateTx.Error)
		}

		if updateTx.RowsAffected > 0 {
			return nil
		}

		err = tx.Model(f.zeroModelPtr()).Create(map[string]any{
			f.relationParentIDColumn: parentID,
			f.relationPkColumn:       pkID,
			"updated_at":             updatedAt,
			"isDeleted":              false,
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

func (f *labelTagMapCommons[Model]) Delete(ctx context.Context, parentID uint, pkID uint) (deleted bool, err error) {
	txDelete := f.db.WithContext(ctx).Delete(
		f.zeroModelPtr(),
		fmt.Sprintf("%s = ? AND %s = ?", f.relationParentIDColumn, f.relationPkColumn),
		parentID, pkID,
	)
	if txDelete.Error != nil {
		return false, fmt.Errorf("failed to delete many2many relation %q: %w", f.relationTableName, txDelete.Error)
	}

	deleted = txDelete.RowsAffected > 0
	return deleted, nil
}

func (f *labelTagMapCommons[Model]) zeroModelPtr() *Model {
	return to.Ptr(to.ZeroValue[Model]())
}

func (f *labelTagMapCommons[Model]) sinceUpdateScope(since time.Time) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where(fmt.Sprintf("%s.updated_at >= ?", f.relationTableName), since)
	}
}
