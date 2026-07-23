package repo

import (
	"gorm.io/gorm"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
)

// applyListActionsTimeFilters applies BRC-114 created_at range filters.
// from is inclusive; to is exclusive — matching the TypeScript implementation.
func applyListActionsTimeFilters(query *gorm.DB, filter entity.ListActionsFilter) *gorm.DB {
	if !filter.TimeFilterRequested {
		return query
	}
	query = query.Where("created_at IS NOT NULL")
	if filter.CreatedAtFrom != nil {
		query = query.Where("created_at >= ?", *filter.CreatedAtFrom)
	}
	if filter.CreatedAtTo != nil {
		query = query.Where("created_at < ?", *filter.CreatedAtTo)
	}
	return query
}
