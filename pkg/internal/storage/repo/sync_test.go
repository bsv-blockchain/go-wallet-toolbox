package repo_test

import (
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/testabilities/dbfixtures"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSync(t *testing.T) {
	//testmode.DevelopmentOnly_SetFileSQLiteMode(t)
	db, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()

	repos := db.CreateRepositories()

	user, err := repos.CreateUser(t.Context(), "test-user", "test-identity-key",
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

	basket, err := repos.FindBasketsForSync(t.Context(), user.UserID)
	require.NoError(t, err)

	require.Len(t, basket, 2)
	defaultBasket := basket[0]
	require.Equal(t, primitives.StringUnder300("default"), defaultBasket.Name)
	require.Equal(t, 1, defaultBasket.BasketID)
}
