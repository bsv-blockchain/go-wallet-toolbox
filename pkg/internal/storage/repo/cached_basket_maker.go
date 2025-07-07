package repo

import (
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type cachedBasketMaker struct {
	tx             *gorm.DB
	userID         int
	checkedBaskets map[string]struct{}
}

func newCachedBasketMaker(tx *gorm.DB, userID int) *cachedBasketMaker {
	return &cachedBasketMaker{
		tx:             tx,
		userID:         userID,
		checkedBaskets: make(map[string]struct{}),
	}
}

func (c *cachedBasketMaker) createIfNotExist(tx *gorm.DB, name string, numberOfDesiredUTXOs int64, minimumDesiredUTXOValue uint64) error {
	if _, ok := c.checkedBaskets[name]; ok {
		return nil
	}

	err := tx.Clauses(clause.OnConflict{
		DoNothing: true,
	}).
		Create(&models.OutputBasket{
			UserID:                  c.userID,
			Name:                    name,
			NumberOfDesiredUTXOs:    numberOfDesiredUTXOs,
			MinimumDesiredUTXOValue: minimumDesiredUTXOValue,
		}).Error

	if err != nil {
		return fmt.Errorf("failed to upsert output basket: %w", err)
	}

	c.checkedBaskets[name] = struct{}{}

	return nil
}
