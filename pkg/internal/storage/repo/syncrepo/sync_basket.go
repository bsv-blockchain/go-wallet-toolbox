package syncrepo

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-softwarelab/common/pkg/slices"
	"gorm.io/gorm"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/genquery"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/scopes"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

type SyncBasket struct {
	db    *gorm.DB
	query *genquery.Query
}

func NewSyncBasket(db *gorm.DB, query *genquery.Query) *SyncBasket {
	return &SyncBasket{db: db, query: query}
}

func (s *SyncBasket) FindBasketsForSync(ctx context.Context, userID int, opts ...queryopts.Options) ([]*wdk.TableOutputBasket, error) {
	filters := append(scopes.FromQueryOpts(opts), scopes.UserID(userID))

	var resultModels []*models.OutputBasket

	err := s.db.WithContext(ctx).
		Model(&models.OutputBasket{}).
		Scopes(filters...).
		Find(&resultModels).Error
	if err != nil {
		return nil, fmt.Errorf("db failed while finding baskets for sync: %w", err)
	}

	return slices.Map(resultModels, s.mapModelToTableOutputBasket), nil
}

func (s *SyncBasket) UpsertOutputBasketForSync(ctx context.Context, entity entity.OutputBasket) (isNew bool, basketID uint, err error) {
	model := models.OutputBasket{
		Timestamps: models.Timestamps{
			CreatedAt: entity.CreatedAt,
			UpdatedAt: entity.UpdatedAt,
		},
		UserID:                  entity.UserID,
		Name:                    entity.Name,
		NumberOfDesiredUTXOs:    entity.NumberOfDesiredUTXOs,
		MinimumDesiredUTXOValue: entity.MinimumDesiredUTXOValue,
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing models.OutputBasket
		existsErr := tx.Model(&models.OutputBasket{}).
			Select("basketId, updated_at").
			Where("userId = ? AND name = ?", entity.UserID, model.Name).
			First(&existing).Error

		if existsErr == nil {
			if !model.UpdatedAt.After(existing.UpdatedAt) {
				basketID = existing.BasketID
				return nil
			}

			updateTx := tx.Model(&models.OutputBasket{}).
				Where("basketId = ? AND updated_at < ?", existing.BasketID, model.UpdatedAt).
				Updates(model)

			if updateTx.Error != nil {
				return fmt.Errorf("failed to update output basket: %w", updateTx.Error)
			}

			basketID = existing.BasketID
			return nil
		}

		if !errors.Is(existsErr, gorm.ErrRecordNotFound) {
			return fmt.Errorf("failed to lookup existing output basket: %w", existsErr)
		}

		if err = tx.Create(&model).Error; err != nil {
			return fmt.Errorf("failed to create output basket: %w", err)
		}

		isNew = true
		basketID = model.BasketID

		return nil
	})
	if err != nil {
		return false, 0, fmt.Errorf("transaction failed: %w", err)
	}

	return isNew, basketID, err
}

func (s *SyncBasket) FindBasketNameByNumIDForSync(ctx context.Context, basketID uint) (string, error) {
	var basketName string

	err := s.db.WithContext(ctx).Model(&models.OutputBasket{}).
		Where("basketId = ?", basketID).
		Select("name").
		Scan(&basketName).Error
	if err != nil {
		return "", fmt.Errorf("failed to find output basket name by basket ID: %w", err)
	}

	return basketName, nil
}

func (s *SyncBasket) mapModelToTableOutputBasket(model *models.OutputBasket) *wdk.TableOutputBasket {
	return &wdk.TableOutputBasket{
		BasketID:  int(model.BasketID),
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
