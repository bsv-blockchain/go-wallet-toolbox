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

// fanoutArgs returns zero-output create-action args carrying a FuelShape.
// With no inputs and no outputs the action is a remix-change action in
// BRC-100 terms — which is exactly what a fan-out is.
func fanoutArgs(shape wdk.ShapedChange) wdk.ValidCreateActionArgs {
	return fixtures.DefaultValidCreateActionArgs(func(args *wdk.ValidCreateActionArgs) {
		args.Outputs = []wdk.ValidCreateActionOutput{}
		args.IsRemixChange = true
		args.Options.FuelShape = &shape
	})
}

func chunkValue(cfg defs.UTXOManagement) uint64 {
	// One chunk must fund a whole leaf fan-out: outputs plus fee headroom.
	return cfg.Throughput.FanoutOutputsPerTx*testDenomination + 1000
}

func TestFanout_ChunkShapeMintsIntoReserve(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	cfg := throughputManagement(defs.SpendPolicyPreferMined)
	activeStorage := given.Provider().WithUTXOManagement(cfg).GORM()

	// given: operator funds in the default basket
	given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)

	// when: a chunk fan-out (reserve-basket shape) funded from default
	shape := wdk.ShapedChange{
		Count:    2,
		Satoshis: primitives.SatoshiValue(chunkValue(cfg)),
		Basket:   primitives.StringUnder300(wdk.BasketNameForReserve),
	}
	result, err := activeStorage.CreateAction(t.Context(), testusers.Alice.AuthID(), fanoutArgs(shape))

	// then: two exact-value chunk outputs into reserve, remainder change to default
	require.NoError(t, err)

	var chunkCount, defaultChangeCount int
	for _, output := range result.Outputs {
		if output.Purpose != "change" {
			continue
		}
		switch to.Value(output.Basket) {
		case primitives.StringUnder300(wdk.BasketNameForReserve):
			chunkCount++
			assert.EqualValues(t, chunkValue(cfg), output.Satoshis)
			assert.NotEmpty(t, output.DerivationSuffix, "chunk outputs must be client-derivable change")
		case primitives.StringUnder300(wdk.BasketNameForChange):
			defaultChangeCount++
		}
	}
	assert.Equal(t, 2, chunkCount)
	assert.Equal(t, 1, defaultChangeCount, "remainder routes to default as a single deterministic change output")

	// and: the shaped rows are persisted as spendable change in the reserve basket
	var outputs []models.Output
	require.NoError(t, activeStorage.Database.DB.
		Where("user_id = ? AND basket_name = ?", testusers.Alice.ID, wdk.BasketNameForReserve).
		Find(&outputs).Error)
	require.Len(t, outputs, 2)
	for _, output := range outputs {
		assert.True(t, output.Change)
		assert.True(t, output.Spendable)
		assert.EqualValues(t, chunkValue(cfg), output.Satoshis)
	}
}

func TestFanout_LeafShapeFundsFromReserve(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	cfg := throughputManagement(defs.SpendPolicyPreferMined)
	cfg.Throughput.FanoutOutputsPerTx = 10 // keep the test tx small
	activeStorage := given.Provider().WithUTXOManagement(cfg).GORM()

	// given: a claimable chunk in the reserve basket and an (irrelevant) default fund
	faucet := given.Faucet(activeStorage, testusers.Alice)
	faucet.TopUp(5_000, pkgtestabilities.WithBasketTopUp(wdk.BasketNameForReserve), pkgtestabilities.WithMinedTopUp())

	// when: a leaf fan-out minting 10 × denomination into the fuel basket
	shape := wdk.ShapedChange{
		Count:    10,
		Satoshis: primitives.SatoshiValue(testDenomination),
		Basket:   primitives.StringUnder300(wdk.BasketNameForFuel),
	}
	result, err := activeStorage.CreateAction(t.Context(), testusers.Alice.AuthID(), fanoutArgs(shape))

	// then: the reserve chunk funds it — never the fuel pool
	require.NoError(t, err)
	require.Len(t, result.Inputs, 1)
	assert.EqualValues(t, 5_000, result.Inputs[0].SourceSatoshis)

	var reserved []models.UserUTXO
	require.NoError(t, activeStorage.Database.DB.
		Where("user_id = ? AND reserved_by_id IS NOT NULL", testusers.Alice.ID).
		Find(&reserved).Error)
	require.Len(t, reserved, 1)
	assert.Equal(t, wdk.BasketNameForReserve, reserved[0].BasketName)

	// and: 10 exact-denomination fuel outputs were created
	var fuelOutputs []models.Output
	require.NoError(t, activeStorage.Database.DB.
		Where("user_id = ? AND basket_name = ?", testusers.Alice.ID, wdk.BasketNameForFuel).
		Find(&fuelOutputs).Error)
	require.Len(t, fuelOutputs, 10)
	for _, output := range fuelOutputs {
		assert.EqualValues(t, testDenomination, output.Satoshis)
		assert.True(t, output.Change)
	}
}

func TestFanout_Validation(t *testing.T) {
	tests := map[string]struct {
		mutate      func(*wdk.ShapedChange, defs.UTXOManagement)
		expectedErr string
	}{
		"wrong denomination for leaf shape": {
			mutate: func(shape *wdk.ShapedChange, _ defs.UTXOManagement) {
				shape.Satoshis = testDenomination + 1
			},
			expectedErr: "must equal the active denomination",
		},
		"chunk below minimum": {
			mutate: func(shape *wdk.ShapedChange, _ defs.UTXOManagement) {
				shape.Basket = primitives.StringUnder300(wdk.BasketNameForReserve)
				shape.Satoshis = 100
			},
			expectedErr: "must be at least",
		},
		"zero count": {
			mutate: func(shape *wdk.ShapedChange, _ defs.UTXOManagement) {
				shape.Count = 0
			},
			expectedErr: "count",
		},
		"count above fanout_outputs_per_tx": {
			mutate: func(shape *wdk.ShapedChange, cfg defs.UTXOManagement) {
				shape.Count = cfg.Throughput.FanoutOutputsPerTx + 1
			},
			expectedErr: "count",
		},
		"unknown basket": {
			mutate: func(shape *wdk.ShapedChange, _ defs.UTXOManagement) {
				shape.Basket = "somewhere-else"
			},
			expectedErr: "must be the pool",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			given, cleanup := testabilities.Given(t)
			defer cleanup()

			cfg := throughputManagement(defs.SpendPolicyPreferMined)
			activeStorage := given.Provider().WithUTXOManagement(cfg).GORM()
			given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)

			shape := wdk.ShapedChange{
				Count:    1,
				Satoshis: primitives.SatoshiValue(testDenomination),
				Basket:   primitives.StringUnder300(wdk.BasketNameForFuel),
			}
			test.mutate(&shape, cfg)

			_, err := activeStorage.CreateAction(t.Context(), testusers.Alice.AuthID(), fanoutArgs(shape))
			require.ErrorContains(t, err, test.expectedErr)
		})
	}
}

func TestFanout_RejectedUnderPrivacyStrategy(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().GORM() // privacy strategy (default)
	given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)

	shape := wdk.ShapedChange{
		Count:    1,
		Satoshis: primitives.SatoshiValue(testDenomination),
		Basket:   primitives.StringUnder300(wdk.BasketNameForFuel),
	}
	_, err := activeStorage.CreateAction(t.Context(), testusers.Alice.AuthID(), fanoutArgs(shape))
	require.ErrorContains(t, err, "requires the throughput")
}
