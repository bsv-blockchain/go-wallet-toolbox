package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/database/scopes"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"gorm.io/gorm"
)

type OutputBaskets struct {
	db *gorm.DB
}

func NewOutputBaskets(db *gorm.DB) *OutputBaskets {
	return &OutputBaskets{db: db}
}

func (o *OutputBaskets) FindBasketByName(ctx context.Context, userID int, name string) (*entity.OutputBasket, error) {
	outputBasket := &models.OutputBasket{}
	err := o.db.WithContext(ctx).
		Scopes(scopes.UserID(userID)).
		Where("name = ?", name).
		First(&outputBasket).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find output basket: %w", err)
	}

	return mapModelToEntityOutputBasket(outputBasket), nil
}

func (o *OutputBaskets) UpsertOutputBasket(ctx context.Context, userID int, basket wdk.BasketConfiguration) (isNew bool, err error) {
	model := models.OutputBasket{
		UserID:                  userID,
		Name:                    string(basket.Name),
		NumberOfDesiredUTXOs:    basket.NumberOfDesiredUTXOs,
		MinimumDesiredUTXOValue: basket.MinimumDesiredUTXOValue,
	}

	err = o.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updateTx := tx.Model(&models.OutputBasket{}).
			Scopes(scopes.UserID(userID)).
			Where("name = ?", basket.Name).
			Updates(map[string]interface{}{
				"number_of_desired_utxos":    basket.NumberOfDesiredUTXOs,
				"minimum_desired_utxo_value": basket.MinimumDesiredUTXOValue,
			})
		if updateTx.Error != nil {
			return fmt.Errorf("failed to update existing output basket: %w", err)
		}

		if updateTx.RowsAffected > 0 {
			return nil
		}

		err := tx.Create(&model).Error
		if err != nil {
			return fmt.Errorf("failed to create new output basket: %w", err)
		}

		isNew = true

		return nil
	})
	if err != nil {
		return false, fmt.Errorf("transaction failed while upserting output basket: %w", err)
	}

	return isNew, nil
}

func mapModelToEntityOutputBasket(model *models.OutputBasket) *entity.OutputBasket {
	return &entity.OutputBasket{
		Name:                    model.Name,
		UserID:                  model.UserID,
		CreatedAt:               model.CreatedAt,
		UpdatedAt:               model.UpdatedAt,
		NumberOfDesiredUTXOs:    model.NumberOfDesiredUTXOs,
		MinimumDesiredUTXOValue: model.MinimumDesiredUTXOValue,
	}
}
