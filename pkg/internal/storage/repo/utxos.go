package repo

import (
	"context"
	"fmt"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/database/scopes"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/paging"
	"gorm.io/gorm"
)

type UTXOs struct {
	db *gorm.DB
}

func NewUTXOs(db *gorm.DB) *UTXOs {
	return &UTXOs{
		db: db,
	}
}

func (u *UTXOs) FindNotReservedUTXOs(ctx context.Context, userID int, basketID int, page *paging.Page, forbiddenOutputIDs []uint) ([]*models.UserUTXO, error) {
	var result []*models.UserUTXO

	query := u.db.WithContext(ctx).Scopes(
		scopes.UserID(userID),
		scopes.BasketID(basketID),
		scopes.Paginate(page),
		notReserved(),
		outputNotIn(forbiddenOutputIDs),
	)

	err := query.Find(&result).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find not reserved UTXOs: %w", err)
	}
	return result, nil
}

func (u *UTXOs) CountUTXOs(ctx context.Context, userID int, basket int) (int64, error) {
	count := int64(0)

	err := u.db.WithContext(ctx).
		Model(&models.UserUTXO{}).
		Scopes(scopes.UserID(userID), scopes.BasketID(basket), notReserved()).
		Count(&count).Error

	return count, err
}

func notReserved() func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("reserved_by_id IS NULL")
	}
}

func outputNotIn(forbiddenOutputIDs []uint) func(*gorm.DB) *gorm.DB {
	if len(forbiddenOutputIDs) == 0 {
		return func(db *gorm.DB) *gorm.DB {
			return db
		}
	}
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("output_id NOT IN ?", forbiddenOutputIDs)
	}
}
