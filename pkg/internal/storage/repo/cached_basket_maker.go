package repo

import (
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
)

type cachedBasketMaker struct {
	tx             *gorm.DB
	userID         int
	checkedBaskets map[string]struct{}
	// shared is the cross-call registry of baskets known to be committed; may be nil.
	shared *basketRegistry
}

func newCachedBasketMaker(tx *gorm.DB, userID int, shared *basketRegistry) *cachedBasketMaker {
	return &cachedBasketMaker{
		tx:             tx,
		userID:         userID,
		checkedBaskets: make(map[string]struct{}),
		shared:         shared,
	}
}

func (c *cachedBasketMaker) createIfNotExist(tx *gorm.DB, name string, numberOfDesiredUTXOs int64, minimumDesiredUTXOValue uint64) error {
	if _, ok := c.checkedBaskets[name]; ok {
		return nil
	}

	if c.shared != nil && c.shared.known(c.userID, name) {
		c.checkedBaskets[name] = struct{}{}
		return nil
	}

	result := tx.Clauses(clause.OnConflict{
		DoNothing: true,
	}).
		Create(&models.OutputBasket{
			UserID:                  c.userID,
			Name:                    name,
			NumberOfDesiredUTXOs:    numberOfDesiredUTXOs,
			MinimumDesiredUTXOValue: minimumDesiredUTXOValue,
		})
	if result.Error != nil {
		return fmt.Errorf("failed to upsert output basket: %w", result.Error)
	}

	// Zero affected rows: the basket already existed and, since a conflicting insert waits for an
	// in-flight one to resolve, as a committed row. Only then is it safe to remember it across
	// calls; a row inserted by this transaction could still be rolled back.
	if c.shared != nil {
		if result.RowsAffected == 0 {
			c.shared.rememberCommitted(c.userID, name, tx.Statement.ConnPool)
		} else {
			c.shared.noteInserted(c.userID, name, tx.Statement.ConnPool)
		}
	}

	c.checkedBaskets[name] = struct{}{}

	return nil
}
