package storage_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

func TestRelinquishOutput(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	activeStorage := given.Provider().GORM()

	// and:
	txSpec, _ := given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)

	// when:
	err := activeStorage.RelinquishOutput(
		t.Context(),
		testusers.Alice.AuthID(),
		wdk.RelinquishOutputArgs{
			Basket: wdk.BasketNameForChange,
			Output: string(primitives.NewOutpointString(txSpec.ID().String(), 0)),
		},
	)

	// then:
	require.NoError(t, err)
	listOutputsResult, err := activeStorage.ListOutputs(t.Context(), testusers.Alice.AuthID(), wdk.ListOutputsArgs{
		Limit:  10,
		Basket: wdk.BasketNameForChange,
	})
	require.NoError(t, err)
	require.Equal(t, 0, int(listOutputsResult.TotalOutputs)) //nolint:gosec // test assertion, TotalOutputs fits in int

	// and:
	_, err = activeStorage.CreateAction(
		t.Context(),
		testusers.Alice.AuthID(),
		fixtures.DefaultValidCreateActionArgs(),
	)
	require.Error(t, err) // make sure that we cannot create an action with relinquished output (got "not enough funds")
}

func TestRelinquishOutputWithoutBasketSpecified(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	activeStorage := given.Provider().GORM()

	// and:
	txSpec, _ := given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)

	// when:
	err := activeStorage.RelinquishOutput(
		t.Context(),
		testusers.Alice.AuthID(),
		wdk.RelinquishOutputArgs{
			Output: string(primitives.NewOutpointString(txSpec.ID().String(), 0)),
		},
	)

	// then:
	require.NoError(t, err)
	listOutputsResult, err := activeStorage.ListOutputs(t.Context(), testusers.Alice.AuthID(), wdk.ListOutputsArgs{
		Limit:  10,
		Basket: wdk.BasketNameForChange,
	})
	require.NoError(t, err)
	require.Equal(t, 0, int(listOutputsResult.TotalOutputs)) //nolint:gosec // test assertion, TotalOutputs fits in int

	// and:
	_, err = activeStorage.CreateAction(
		t.Context(),
		testusers.Alice.AuthID(),
		fixtures.DefaultValidCreateActionArgs(),
	)
	require.Error(t, err) // make sure that we cannot create an action with relinquished output (got "not enough funds")
}

func TestRelinquishNotExistingOutput(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	activeStorage := given.Provider().GORM()

	// when:
	err := activeStorage.RelinquishOutput(
		t.Context(),
		testusers.Alice.AuthID(),
		wdk.RelinquishOutputArgs{
			Output: fixtures.MockOutpoint,
		},
	)

	// then:
	require.Error(t, err)
}

func TestRelinquishOutputOneOfTwo(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	activeStorage := given.Provider().GORM()

	// and:
	faucet := given.Faucet(activeStorage, testusers.Alice)
	faucet.TopUp(500_000)
	txSpec, _ := faucet.TopUp(100_000)

	// when:
	err := activeStorage.RelinquishOutput(
		t.Context(),
		testusers.Alice.AuthID(),
		wdk.RelinquishOutputArgs{
			Output: string(primitives.NewOutpointString(txSpec.ID().String(), 0)),
		},
	)

	// then:
	require.NoError(t, err)
	listOutputsResult, err := activeStorage.ListOutputs(t.Context(), testusers.Alice.AuthID(), wdk.ListOutputsArgs{
		Limit:  10,
		Basket: wdk.BasketNameForChange,
	})
	require.NoError(t, err)
	require.Equal(t, 1, int(listOutputsResult.TotalOutputs)) //nolint:gosec // safe: TotalOutputs is a small count value
}

func TestRelinquishOutputWithNotMatchingBasket(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	activeStorage := given.Provider().GORM()

	// and:
	txSpec, _ := given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)

	// when:
	err := activeStorage.RelinquishOutput(
		t.Context(),
		testusers.Alice.AuthID(),
		wdk.RelinquishOutputArgs{
			Basket: "other-basket",
			Output: string(primitives.NewOutpointString(txSpec.ID().String(), 0)),
		},
	)

	// then:
	require.Error(t, err)
}
