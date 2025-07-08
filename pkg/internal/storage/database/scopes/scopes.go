package scopes

import (
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Paginate is a Scope function that handles pagination.
func Paginate(page *queryopts.Paging) func(db *gorm.DB) *gorm.DB {
	page.ApplyDefaults()
	return func(db *gorm.DB) *gorm.DB {
		orderClause := clause.OrderByColumn{
			Column: clause.Column{Name: page.SortBy},
			Desc:   page.IsDesc(),
		}
		return db.Order(orderClause).Offset(page.Offset).Limit(page.Limit)
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

func BasketName(basketName string) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("basket_name = ?", basketName)
	}
}

func Since(since *queryopts.Since) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		since.ApplyDefaults()
		return db.Where(
			&clause.Gte{
				Column: clause.Column{Name: since.Field, Table: since.TableName},
				Value:  since.Time,
			},
		)
	}
}

func FromQueryOpts(opts []queryopts.Options) []func(*gorm.DB) *gorm.DB {
	options := queryopts.MergeOptions(opts)

	var sc []func(*gorm.DB) *gorm.DB
	if options.Page != nil {
		sc = append(sc, Paginate(options.Page))
	}
	if options.Since != nil {
		sc = append(sc, Since(options.Since))
	}

	return sc
}
