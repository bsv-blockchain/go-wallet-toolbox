package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/database/scopes"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

func (o *OutputBaskets) UpsertOutputBasket(ctx context.Context, userID int, basket wdk.BasketConfiguration) (*entity.OutputBasket, error) {
	model := models.OutputBasket{
		UserID:                  userID,
		Name:                    string(basket.Name),
		NumberOfDesiredUTXOs:    basket.NumberOfDesiredUTXOs,
		MinimumDesiredUTXOValue: basket.MinimumDesiredUTXOValue,
		UpdatedAt:               time.Now(),
	}
	tx := o.db.WithContext(ctx).
		Model(&models.OutputBasket{}).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "name"}},
			DoUpdates: clause.AssignmentColumns([]string{"number_of_desired_utxos", "minimum_desired_utxo_value", "updated_at"}),
		}).
		Create(&model)

	if tx.Error != nil {
		return nil, fmt.Errorf("failed to upsert output basket: %w", tx.Error)
	}

	return mapModelToEntityOutputBasket(&model), nil
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
