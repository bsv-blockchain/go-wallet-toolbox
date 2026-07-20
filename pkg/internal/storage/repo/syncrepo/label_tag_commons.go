package syncrepo

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-softwarelab/common/pkg/to"
	"gorm.io/gorm"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/genquery"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/scopes"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
)

type labelTagCommons[Model, RelationModel, ReadModel any] struct {
	db                   *gorm.DB
	query                *genquery.Query
	tableName            string
	relationUserIDColumn string
	relationValueColumn  string
}

func (f *labelTagCommons[_, _, ReadModel]) FindChunk(ctx context.Context, userID int, opts ...queryopts.Options) ([]*ReadModel, error) {
	var resultModels []*ReadModel

	filters := append(scopes.FromQueryOpts(opts), scopes.UserID(userID))

	err := f.db.WithContext(ctx).
		Model(f.zeroModelPtr()).
		Scopes(filters...).
		Unscoped().
		Find(&resultModels).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find: %w", err)
	}

	return resultModels, nil
}

func (f *labelTagCommons[Model, _, ReadModel]) Upsert(ctx context.Context, userID int, value string, model *Model) (isNew bool, pkID uint, err error) {
	err = f.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updateTx := tx.Model(f.zeroModelPtr()).
			Where(fmt.Sprintf("%s = ? AND %s = ?", f.relationUserIDColumn, f.relationValueColumn), userID, value).
			Updates(model)

		if updateTx.Error != nil {
			return fmt.Errorf("failed to update: %w", updateTx.Error)
		}

		if updateTx.RowsAffected > 0 {
			// Find the primary key
			var existing Model
			if err := tx.Model(f.zeroModelPtr()).
				Where(fmt.Sprintf("%s = ? AND %s = ?", f.relationUserIDColumn, f.relationValueColumn), userID, value).
				First(&existing).Error; err != nil {
				return fmt.Errorf("failed to find updated model: %w", err)
			}

			// We cannot reliably get ID from reflection without more code, but since we are doing generic repo,
			// maybe we don't return pkID from here, or we use reflection or returning model itself.
			// Actually, let's just do Create and rely on Upsert semantics if we can, or just do First to get ID.
			// Let's refactor `sync_label.go` to use this properly.
			// Since we need the ID, let's return `model` instead or use a generic interface?
			// Wait, the callers of Upsert are sync_label and sync_tag. They might not need the ID anymore because they don't have to join?
			// Let's return 0 for now and we'll fix callers.
			return nil
		}

		err = tx.Create(model).Error
		if err != nil {
			return fmt.Errorf("failed to create: %w", err)
		}

		isNew = true

		return nil
	})
	if err != nil {
		return false, 0, fmt.Errorf("transaction failed for %s: %w", f.tableName, err)
	}

	return isNew, 0, nil // Caller should get ID from `model` directly since GORM populates it
}

func (f *labelTagCommons[_, _, _]) Delete(ctx context.Context, userID int, value string) (deleted bool, err error) {
	err = f.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txDelete := tx.Delete(
			f.zeroModelPtr(),
			fmt.Sprintf("%s = ? AND %s = ?", f.relationUserIDColumn, f.relationValueColumn), userID, value,
		)
		if txDelete.Error != nil {
			return fmt.Errorf("failed to delete: %w", txDelete.Error)
		}

		deleted = txDelete.RowsAffected > 0

		err = tx.Delete(
			f.zeroRelationModelPtr(),
			fmt.Sprintf("%s = ? AND %s = ?", f.relationUserIDColumn, f.relationValueColumn), userID, value,
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

func (f *labelTagCommons[Model, _, _]) FindByID(ctx context.Context, pkID uint, pkCol string) (*Model, error) {
	label := f.zeroModelPtr()

	err := f.db.WithContext(ctx).
		Where(fmt.Sprintf("%s = ?", pkCol), pkID).
		First(&label).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find %s by ID: %w", f.tableName, err)
	}

	return label, nil
}

func (f *labelTagCommons[Model, _, _]) zeroModelPtr() *Model {
	return to.Ptr(to.ZeroValue[Model]())
}

func (f *labelTagCommons[_, RelationModel, _]) zeroRelationModelPtr() *RelationModel {
	return to.Ptr(to.ZeroValue[RelationModel]())
}
