package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/database/scopes"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/go-softwarelab/common/pkg/slices"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type OutputBaskets struct {
	db *gorm.DB
}

func NewOutputBaskets(db *gorm.DB) *OutputBaskets {
	return &OutputBaskets{db: db}
}

func (o *OutputBaskets) FindBasketByName(ctx context.Context, userID int, name string) (*wdk.TableOutputBasket, error) {
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

	return mapModelToTableOutputBasket(outputBasket), nil
}

func (o *OutputBaskets) FindBasketsByUserID(ctx context.Context, userID int, opts ...queryopts.Options) ([]*wdk.TableOutputBasket, error) {
	var outputBaskets []*models.OutputBasket
	err := o.db.WithContext(ctx).
		Scopes(scopes.UserID(userID)).
		Scopes(scopes.FromQueryOpts(opts)...).
		Find(&outputBaskets).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find output baskets: %w", err)
	}

	return slices.Map(outputBaskets, mapModelToTableOutputBasket), nil
}

func (o *OutputBaskets) UpsertOutputBasket(ctx context.Context, userID int, basket wdk.BasketConfiguration) error {
	err := o.db.WithContext(ctx).
		Model(&models.OutputBasket{}).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "name"}},
			DoUpdates: clause.AssignmentColumns([]string{"number_of_desired_utxos", "minimum_desired_utxo_value"}),
		}).
		Create(&models.OutputBasket{
			UserID:                  userID,
			Name:                    string(basket.Name),
			NumberOfDesiredUTXOs:    basket.NumberOfDesiredUTXOs,
			MinimumDesiredUTXOValue: basket.MinimumDesiredUTXOValue,
		}).Error

	if err != nil {
		return fmt.Errorf("failed to upsert output basket: %w", err)
	}

	return nil
}

func mapModelToTableOutputBasket(model *models.OutputBasket) *wdk.TableOutputBasket {
	return &wdk.TableOutputBasket{
		BasketNumID: model.Num,
		UserID:      model.UserID,
		CreatedAt:   model.CreatedAt,
		UpdatedAt:   model.UpdatedAt,
		BasketConfiguration: wdk.BasketConfiguration{
			Name:                    primitives.StringUnder300(model.Name),
			NumberOfDesiredUTXOs:    model.NumberOfDesiredUTXOs,
			MinimumDesiredUTXOValue: model.MinimumDesiredUTXOValue,
		},
	}
}
