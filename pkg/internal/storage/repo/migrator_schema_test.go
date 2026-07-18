package repo_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/dbfixtures"
)

// The funder hot path filters UserUTXO rows by basket_name (and reserved_by_id)
// when selecting spendable UTXOs. A gorm tag typo ("not null,index" — a comma
// instead of a semicolon) meant GORM treated the whole string as a single
// unrecognized directive, so AutoMigrate created neither the NOT NULL
// constraint nor the index the hot path depends on (W1-5).

func TestMigrationCreatesUserUTXOBasketNameIndex(t *testing.T) {
	// given:
	db, cleanup := dbfixtures.TestDatabase(t) // Migrate already ran inside
	defer cleanup()

	migrator := db.DB.Migrator()

	// then: index exists
	require.True(t, migrator.HasIndex(&models.UserUTXO{}, "BasketName"),
		"basket_name index missing — funder hot path filters unindexed")

	// and: column is NOT NULL
	columns, err := migrator.ColumnTypes(&models.UserUTXO{})
	require.NoError(t, err)
	for _, c := range columns {
		if c.Name() == "basket_name" {
			nullable, ok := c.Nullable()
			require.True(t, ok)
			require.False(t, nullable, "basket_name must be NOT NULL")
			return
		}
	}
	t.Fatal("basket_name column not found")
}
