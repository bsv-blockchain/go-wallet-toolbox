package syncrepo

import (
	"context"
	"fmt"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/genquery"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/scopes"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/go-softwarelab/common/pkg/slices"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SyncBasket struct {
	db *gorm.DB
}

func NewSyncBasket(db *gorm.DB) *SyncBasket {
	return &SyncBasket{db: db}
}

type OutputBasketWithNum struct {
	models.OutputBasket
	NumID int
}

func (s *SyncBasket) tableName() string {
	return genquery.OutputBasket.TableName()
}

func (s *SyncBasket) stringIDClause() string {
	return "CONCAT(user_id, '.', name)"
}

func (s *SyncBasket) FindBasketsForSync(ctx context.Context, userID int, opts ...queryopts.Options) ([]*wdk.TableOutputBasket, error) {
	filters := append(scopes.FromQueryOpts(opts), scopes.UserID(userID))

	resultModels, err := findChunk[models.OutputBasket, OutputBasketWithNum](ctx, s.db, s.tableName(), s.stringIDClause(), filters)
	if err != nil {
		return nil, fmt.Errorf("failed to find output baskets for sync: %w", err)
	}

	return slices.Map(resultModels, s.mapModelToTableOutputBasket), nil
}

func (s *SyncBasket) UpsertOutputBasketForSync(ctx context.Context, entity entity.OutputBasket) (isNew bool, basketNumID uint, err error) {
	model := models.OutputBasket{
		CreatedAt:               entity.CreatedAt,
		UpdatedAt:               entity.UpdatedAt,
		UserID:                  entity.UserID,
		Name:                    entity.Name,
		NumberOfDesiredUTXOs:    entity.NumberOfDesiredUTXOs,
		MinimumDesiredUTXOValue: entity.MinimumDesiredUTXOValue,
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		numID, err := s.saveNumericIDForOutputBasket(ctx, tx, entity.UserID, entity.Name)
		if err != nil {
			return err
		}

		basketNumID = numID

		updateTx := tx.Model(&models.OutputBasket{}).
			Where("user_id = ? AND name = ?", entity.UserID, model.Name).
			Updates(model)

		if updateTx.Error != nil {
			return fmt.Errorf("failed to update output basket: %w", updateTx.Error)
		}

		if updateTx.RowsAffected > 0 {
			return nil
		}

		err = tx.Create(&model).Error
		if err != nil {
			return fmt.Errorf("failed to create output basket: %w", err)
		}

		isNew = true

		return nil
	})

	if err != nil {
		return false, 0, fmt.Errorf("transaction failed: %w", err)
	}

	return
}

func (s *SyncBasket) FindBasketNameByNumIDForSync(ctx context.Context, basketNumID uint) (string, error) {
	var basketName string

	err := s.db.WithContext(ctx).Model(&models.OutputBasket{}).
		Scopes(joinWithNumericIDLookupScope(s.stringIDClause(), s.tableName(), clause.InnerJoin)).
		Where("num.num_id = ?", basketNumID).
		Select("name").
		Scan(&basketName).Error
	if err != nil {
		return "", fmt.Errorf("failed to find output basket name by numeric ID: %w", err)
	}

	return basketName, nil
}

func (s *SyncBasket) saveNumericIDForOutputBasket(ctx context.Context, tx *gorm.DB, userID int, basketName string) (uint, error) {
	stringID := fmt.Sprintf("%d.%s", userID, basketName)

	err := saveNumericIDLookup(ctx, tx, s.tableName(), stringID)
	if err != nil {
		return 0, fmt.Errorf("failed to save numeric ID lookup for output basket: %w", err)
	}

	return findNumericIDLookup(ctx, tx, s.tableName(), stringID)
}

func (s *SyncBasket) mapModelToTableOutputBasket(model *OutputBasketWithNum) *wdk.TableOutputBasket {
	return &wdk.TableOutputBasket{
		BasketID:  model.NumID,
		UserID:    model.UserID,
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.UpdatedAt,
		BasketConfiguration: wdk.BasketConfiguration{
			Name:                    primitives.StringUnder300(model.Name),
			NumberOfDesiredUTXOs:    model.NumberOfDesiredUTXOs,
			MinimumDesiredUTXOValue: model.MinimumDesiredUTXOValue,
		},
	}
}
