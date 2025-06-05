package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/database/scopes"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/slices"
	"gorm.io/gorm"
)

type OutputBaskets struct {
	db *gorm.DB
}

func NewOutputBaskets(db *gorm.DB) *OutputBaskets {
	return &OutputBaskets{db: db}
}

func (u *OutputBaskets) FindBasketByName(ctx context.Context, userID int, name string) (*wdk.TableOutputBasket, error) {
	outputBasket := &models.OutputBasket{}
	err := u.db.WithContext(ctx).First(&outputBasket, "user_id = ? AND name = ?", userID, name).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find output basket: %w", err)
	}

	return mapToTableOutputBasket(outputBasket), nil
}

func (u *OutputBaskets) FindBasketsByUserID(ctx context.Context, userID int, opts ...queryopts.QueryOptsUnion) ([]*wdk.TableOutputBasket, error) {
	var outputBaskets []*models.OutputBasket
	err := u.db.WithContext(ctx).
		Scopes(scopes.UserID(userID)).
		Scopes(scopes.FromQueryOpts(opts...)...).
		Find(&outputBaskets).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find output baskets: %w", err)
	}

	return slices.Map(outputBaskets, mapToTableOutputBasket), nil
}

func mapToTableOutputBasket(basket *models.OutputBasket) *wdk.TableOutputBasket {
	if basket == nil {
		return nil
	}
	return &wdk.TableOutputBasket{
		BasketID:  basket.BasketID,
		UserID:    basket.UserID,
		CreatedAt: basket.CreatedAt,
		UpdatedAt: basket.UpdatedAt,

		BasketConfiguration: wdk.BasketConfiguration{
			Name:                    basket.Name,
			NumberOfDesiredUTXOs:    basket.NumberOfDesiredUTXOs,
			MinimumDesiredUTXOValue: basket.MinimumDesiredUTXOValue,
		},
	}
}
