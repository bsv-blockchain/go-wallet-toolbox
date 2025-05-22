package repo

import (
	"fmt"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/database/models"
	"gorm.io/gorm"
)

type cachedBasketMaker struct {
	tx       *gorm.DB
	userID   int
	nameToID map[string]*int
}

func newCachedBasketMaker(tx *gorm.DB, userID int) *cachedBasketMaker {
	return &cachedBasketMaker{
		tx:       tx,
		userID:   userID,
		nameToID: make(map[string]*int),
	}
}

func (c *cachedBasketMaker) findOrCreate(tx *gorm.DB, name string, numberOfDesiredUTXOs int64, minimumDesiredUTXOValue uint64) (*int, error) {
	if cachedID, ok := c.nameToID[name]; ok {
		return cachedID, nil
	}

	var basket models.OutputBasket
	err := tx.
		Where(models.OutputBasket{UserID: c.userID, Name: name}).
		Attrs(models.OutputBasket{NumberOfDesiredUTXOs: numberOfDesiredUTXOs, MinimumDesiredUTXOValue: minimumDesiredUTXOValue}).
		FirstOrCreate(&basket).
		Error
	if err != nil {
		return nil, fmt.Errorf("failed to find or create output basket: %w", err)
	}

	return &basket.BasketID, nil
}
