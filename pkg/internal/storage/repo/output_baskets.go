package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
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

	return &wdk.TableOutputBasket{
		BasketID:  outputBasket.BasketID,
		UserID:    outputBasket.UserID,
		CreatedAt: outputBasket.CreatedAt,
		UpdatedAt: outputBasket.UpdatedAt,

		BasketConfiguration: wdk.BasketConfiguration{
			Name:                    outputBasket.Name,
			NumberOfDesiredUTXOs:    outputBasket.NumberOfDesiredUTXOs,
			MinimumDesiredUTXOValue: outputBasket.MinimumDesiredUTXOValue,
		},
	}, nil
}
