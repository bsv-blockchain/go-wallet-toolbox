package storage_test

import (
	"testing"

	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	pkgtestabilities "github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

const testDenomination = 240

func throughputManagement(policy defs.SpendPolicy) defs.UTXOManagement {
	cfg := defs.DefaultUTXOManagement()
	cfg.Strategy = defs.StrategyThroughput
	cfg.Throughput.DenominationSatoshis = testDenomination
	cfg.Throughput.SpendPolicy = policy
	return cfg
}

// smallActionArgs returns create-action args whose need (output + fee at the
// fixture's 1 sat/kb rate) fits within a single fuel claim.
func smallActionArgs(satoshis uint64) wdk.ValidCreateActionArgs {
	return fixtures.DefaultValidCreateActionArgs(func(args *wdk.ValidCreateActionArgs) {
		args.Outputs[0].Satoshis = primitives.SatoshiValue(satoshis)
	})
}

func TestThroughputFunding_ExactMatchSingleClaim(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given: a throughput-mode provider with a fuel pool of exact denominations
	activeStorage := given.Provider().
		WithUTXOManagement(throughputManagement(defs.SpendPolicyPreferMined)).
		GORM()

	faucet := given.Faucet(activeStorage, testusers.Alice)
	for range 3 {
		faucet.TopUp(testDenomination, pkgtestabilities.WithBasketTopUp(wdk.BasketNameForFuel), pkgtestabilities.WithMinedTopUp())
	}

	// when: a small action that one fuel claim covers
	result, err := activeStorage.CreateAction(t.Context(), testusers.Alice.AuthID(), smallActionArgs(200))

	// then:
	require.NoError(t, err)
	require.Len(t, result.Inputs, 1, "exactly one fuel claim expected")
	assert.EqualValues(t, testDenomination, result.Inputs[0].SourceSatoshis)

	// and: the claim came from the fuel basket
	var reserved []models.UserUTXO
	require.NoError(t, activeStorage.Database.DB.
		Where("user_id = ? AND reserved_by_id IS NOT NULL", testusers.Alice.ID).
		Find(&reserved).Error)
	require.Len(t, reserved, 1)
	assert.Equal(t, wdk.BasketNameForFuel, reserved[0].BasketName)

	// and: change (overshoot) routes to the default basket, never to fuel
	for _, output := range result.Outputs {
		if output.Purpose == "change" {
			assert.Equal(t, primitives.StringUnder300(wdk.BasketNameForChange), to.Value(output.Basket))
		}
	}
}

func TestThroughputFunding_PackedActionMultiClaims(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().
		WithUTXOManagement(throughputManagement(defs.SpendPolicyPreferMined)).
		GORM()

	faucet := given.Faucet(activeStorage, testusers.Alice)
	for range 10 {
		faucet.TopUp(testDenomination, pkgtestabilities.WithBasketTopUp(wdk.BasketNameForFuel), pkgtestabilities.WithMinedTopUp())
	}

	// when: an action too large for a single claim
	result, err := activeStorage.CreateAction(t.Context(), testusers.Alice.AuthID(), smallActionArgs(1000))

	// then: several fuel claims fund it
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(result.Inputs), 2)
	for _, input := range result.Inputs {
		assert.EqualValues(t, testDenomination, input.SourceSatoshis)
	}
}

func TestThroughputFunding_FallsBackToDefaultBasket(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given: throughput mode with an EMPTY fuel pool but a funded default basket
	activeStorage := given.Provider().
		WithUTXOManagement(throughputManagement(defs.SpendPolicyPreferMined)).
		GORM()

	given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)

	// when:
	result, err := activeStorage.CreateAction(t.Context(), testusers.Alice.AuthID(), smallActionArgs(200))

	// then: the request is funded from the default basket
	require.NoError(t, err)
	require.NotEmpty(t, result.Inputs)

	var reserved []models.UserUTXO
	require.NoError(t, activeStorage.Database.DB.
		Where("user_id = ? AND reserved_by_id IS NOT NULL", testusers.Alice.ID).
		Find(&reserved).Error)
	require.NotEmpty(t, reserved)
	for _, utxo := range reserved {
		assert.Equal(t, wdk.BasketNameForChange, utxo.BasketName)
	}
}

func TestThroughputFunding_MinedOnlyPolicyRefusesUnprovenFuel(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given: mined_only policy and a fuel pool that is only unproven
	activeStorage := given.Provider().
		WithUTXOManagement(throughputManagement(defs.SpendPolicyMinedOnly)).
		GORM()

	given.Faucet(activeStorage, testusers.Alice).
		TopUp(testDenomination, pkgtestabilities.WithBasketTopUp(wdk.BasketNameForFuel))

	// when:
	_, err := activeStorage.CreateAction(t.Context(), testusers.Alice.AuthID(), smallActionArgs(200))

	// then: unproven fuel is not claimable, and the default basket is empty
	require.ErrorIs(t, err, wdk.ErrNotEnoughFunds)
}

func TestThroughputFunding_PreferMinedClaimsUnprovenFuel(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().
		WithUTXOManagement(throughputManagement(defs.SpendPolicyPreferMined)).
		GORM()

	given.Faucet(activeStorage, testusers.Alice).
		TopUp(testDenomination, pkgtestabilities.WithBasketTopUp(wdk.BasketNameForFuel))

	// when:
	result, err := activeStorage.CreateAction(t.Context(), testusers.Alice.AuthID(), smallActionArgs(200))

	// then:
	require.NoError(t, err)
	require.Len(t, result.Inputs, 1)
	assert.EqualValues(t, testDenomination, result.Inputs[0].SourceSatoshis)
}
