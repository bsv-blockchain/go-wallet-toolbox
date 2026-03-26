package repo_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/dbfixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

func TestSyncWithNumericIDLookup(t *testing.T) {
	// given:
	db, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()

	repos := db.CreateRepositories()

	user, err := repos.CreateUser(t.Context(), testusers.Alice.IdentityKey(t), "test-identity-key",
		wdk.BasketConfiguration{
			Name:                    "default",
			NumberOfDesiredUTXOs:    1,
			MinimumDesiredUTXOValue: 1000,
		}, wdk.BasketConfiguration{
			Name:                    "secondary",
			NumberOfDesiredUTXOs:    2,
			MinimumDesiredUTXOValue: 2000,
		},
	)
	require.NoError(t, err)

	page := queryopts.Paging{
		Limit:  10,
		Offset: 0,
		SortBy: "number_of_desired_utxos",
		Sort:   "asc",
	}

	// when:
	baskets, err := repos.FindBasketsForSync(t.Context(), user.ID, queryopts.WithPage(page))

	// then:
	require.NoError(t, err)

	require.Len(t, baskets, 2)
	defaultBasket := baskets[0]
	require.Equal(t, primitives.StringUnder300("default"), defaultBasket.Name)
	require.Equal(t, 1, defaultBasket.BasketID)

	secondaryBasket := baskets[1]
	require.Equal(t, primitives.StringUnder300("secondary"), secondaryBasket.Name)
	require.Equal(t, 2, secondaryBasket.BasketID)

	// given:
	_, err = repos.UpsertOutputBasket(t.Context(), user.ID, wdk.BasketConfiguration{
		Name:                    "other",
		NumberOfDesiredUTXOs:    3,
		MinimumDesiredUTXOValue: 3000,
	})
	require.NoError(t, err)

	// when:
	baskets, err = repos.FindBasketsForSync(t.Context(), user.ID, queryopts.WithPage(page))

	// then:
	require.NoError(t, err)
	require.Len(t, baskets, 3)
	require.Equal(t, 1, baskets[0].BasketID)
	require.Equal(t, 2, baskets[1].BasketID)
	require.Equal(t, 5, baskets[2].BasketID)
}
