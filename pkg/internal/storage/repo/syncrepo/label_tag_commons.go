package syncrepo

import (
	"context"
	"errors"
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/genquery"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/scopes"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/go-softwarelab/common/pkg/to"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type labelTagCommons[Model, RelationModel, ReadModel any] struct {
	db                   *gorm.DB
	query                *genquery.Query
	tableName            string
	relationUserIDColumn string
	relationNameColumn   string
}

func (f *labelTagCommons[_, _, ReadModel]) FindChunk(ctx context.Context, userID int, opts ...queryopts.Options) ([]*ReadModel, error) {
	var resultModels []*ReadModel

	err := f.db.Transaction(func(tx *gorm.DB) error {
		filters := append(scopes.FromQueryOpts(opts), scopes.UserID(userID))

		err := upsertNumericIDLookup(ctx, f.db, tx, f.query, func(db *gorm.DB) *gorm.DB {
			return db.
				Select(fmt.Sprintf("?, %s", f.stringIDClause()), f.tableName).
				Scopes(filters...).
				Unscoped().
				Find(f.zeroModelPtr())
		})
		if err != nil {
			return err
		}

		err = tx.WithContext(ctx).
			Model(f.zeroModelPtr()).
			Select("*").
			Scopes(filters...).
			Scopes(joinWithNumericIDLookupScope(f.query, f.stringIDClause(), f.tableName, clause.InnerJoin)).
			Unscoped().
			Find(&resultModels).Error
		if err != nil {
			return fmt.Errorf("failed to find: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("transaction failed for %s: %w", f.tableName, err)
	}

	return resultModels, nil
}

func (f *labelTagCommons[Model, _, ReadModel]) Upsert(ctx context.Context, userID int, name string, model *Model) (isNew bool, numID uint, err error) {
	err = f.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		numID, err = f.saveNumericID(ctx, tx, userID, name)
		if err != nil {
			return err
		}

		updateTx := tx.Model(f.zeroModelPtr()).
			Where("user_id = ? AND name = ?", userID, name).
			Updates(model)

		if updateTx.Error != nil {
			return fmt.Errorf("failed to update: %w", updateTx.Error)
		}

		if updateTx.RowsAffected > 0 {
			return nil
		}

		err = tx.Create(&model).Error
		if err != nil {
			return fmt.Errorf("failed to create: %w", err)
		}

		isNew = true

		return nil
	})

	if err != nil {
		return false, 0, fmt.Errorf("transaction failed for %s: %w", f.tableName, err)
	}

	return isNew, numID, nil
}

func (f *labelTagCommons[_, _, _]) Delete(ctx context.Context, userID int, name string) (deleted bool, err error) {
	err = f.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txDelete := tx.Delete(
			f.zeroModelPtr(),
			"user_id = ? AND name = ?", userID, name,
		)
		if txDelete.Error != nil {
			return fmt.Errorf("failed to delete: %w", txDelete.Error)
		}

		deleted = txDelete.RowsAffected > 0

		err = tx.Delete(
			f.zeroRelationModelPtr(),
			fmt.Sprintf("%s = ? AND %s = ?", f.relationUserIDColumn, f.relationNameColumn), userID, name,
		).Error
		if err != nil {
			return fmt.Errorf("failed to delete map entries: %w", err)
		}

		return nil
	})

	if err != nil {
		return false, fmt.Errorf("transaction failed for %s: %w", f.tableName, err)
	}

	return deleted, nil
}

func (f *labelTagCommons[Model, _, _]) FindByNumID(ctx context.Context, numID uint) (*Model, error) {
	label := f.zeroModelPtr()

	err := f.db.WithContext(ctx).
		Scopes(joinWithNumericIDLookupScope(f.query, f.stringIDClause(), f.tableName, clause.InnerJoin)).
		Where("num.num_id = ?", numID).
		First(&label).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find %s by numeric ID: %w", f.tableName, err)
	}

	return label, nil
}

func (f *labelTagCommons[_, _, _]) saveNumericID(ctx context.Context, tx *gorm.DB, userID int, name string) (uint, error) {
	stringID := fmt.Sprintf("%d.%s", userID, name)

	err := saveNumericIDLookup(ctx, tx, f.tableName, stringID)
	if err != nil {
		return 0, fmt.Errorf("failed to save numeric ID lookup: %w", err)
	}

	return findNumericIDLookup(ctx, tx, f.tableName, stringID)
}

func (f *labelTagCommons[Model, _, _]) zeroModelPtr() *Model {
	return to.Ptr(to.ZeroValue[Model]())
}

func (f *labelTagCommons[_, RelationModel, _]) zeroRelationModelPtr() *RelationModel {
	return to.Ptr(to.ZeroValue[RelationModel]())
}

func (f *labelTagCommons[_, _, _]) stringIDClause() string {
	return "CONCAT(user_id, '.', name)"
}
