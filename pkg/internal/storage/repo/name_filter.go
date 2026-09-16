package repo

import (
	"gorm.io/gorm"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
)

// labelledWith restricts a bsv_transactions query to the user's transactions
// carrying the given labels: any of them, or under QueryModeAll every one.
func labelledWith(tx *gorm.DB, userID int, labels []string, mode defs.QueryMode) func(*gorm.DB) *gorm.DB {
	return associatedWith(labels, mode, func(names []string) *gorm.DB {
		return tx.Model(&models.TransactionLabel{}).
			Select("transaction_id").
			Where("label_name IN ?", names).
			Where("label_user_id = ?", userID)
	})
}

// taggedWith restricts a bsv_outputs query to the user's outputs carrying the
// given tags: any of them, or under QueryModeAll every one.
func taggedWith(tx *gorm.DB, userID int, tags []string, mode defs.QueryMode) func(*gorm.DB) *gorm.DB {
	return associatedWith(tags, mode, func(names []string) *gorm.DB {
		return tx.Model(&models.OutputTag{}).
			Select("output_id").
			Where("tag_name IN ?", names).
			Where("tag_user_id = ?", userID)
	})
}

// associatedWith builds the name filter shared by labels and tags. idsNamed
// selects the ids of the rows associated with any of the names it is given.
//
// QueryModeAll intersects one subquery per distinct name. It used to be a single
// subquery over all the names, grouped by id and kept where the distinct count
// matched. That form has to read, sort and group every association of the
// broadest name: EXISTS-style probes cannot short-circuit a GROUP BY. A label
// most rows carry - the Enterprise Wallet puts a kind label on every action and
// checks for an operation id plus that kind before each mint, transfer and burn -
// then costs the whole label on every call. On a 51,000-action wallet that was
// 51,006 index rows sorted on disk, 41 ms per query; intersected, Postgres
// starts from the selective name and probes the other through the name index,
// 0.2 ms, regardless of table size.
//
// Deduplicating also makes a repeated name harmless. The count form required as
// many distinct names as the request listed, so a repeat could never match.
func associatedWith(names []string, mode defs.QueryMode, idsNamed func(names []string) *gorm.DB) func(*gorm.DB) *gorm.DB {
	return func(query *gorm.DB) *gorm.DB {
		if mode != defs.QueryModeAll {
			return query.Where("id IN (?)", idsNamed(names))
		}

		seen := make(map[string]struct{}, len(names))
		for _, name := range names {
			if _, dup := seen[name]; dup {
				continue
			}
			seen[name] = struct{}{}
			query = query.Where("id IN (?)", idsNamed([]string{name}))
		}
		return query
	}
}
