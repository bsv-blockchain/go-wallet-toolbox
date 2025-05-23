package scopes

import (
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/paging"
	"gorm.io/gorm"
)

// Paginate is a Scope function that handle pagination.
func Paginate(page *paging.Page) func(db *gorm.DB) *gorm.DB {
	page.ApplyDefaults()
	return func(db *gorm.DB) *gorm.DB {
		return db.Order(page.SortBy + " " + page.Sort).Offset(page.Offset).Limit(page.Limit)
	}
}

// UserID is a scope function that filters by user ID.
func UserID(id int) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("user_id = ?", id)
	}
}

func Preload(name string) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Preload(name)
	}
}

// BasketID is a scope function that filters by basket_id.
func BasketID(basketID int) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("basket_id = ?", basketID)
	}
}
