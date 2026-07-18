package storage_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func throughputUTXOManagement() defs.UTXOManagement {
	cfg := defs.DefaultUTXOManagement()
	cfg.Strategy = defs.StrategyThroughput
	// Fixture fee model is 1 sat/kb; keep the denomination explicit so tests
	// are independent of derivation.
	cfg.Throughput.DenominationSatoshis = 240
	return cfg
}

func TestFindOrInsertUser_ThroughputSeedsFuelAndReserveBaskets(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	userIdentityKey := "03f17660f611ce531402a2ce1e070380b6fde57aca211d707bfab27bce42d86beb"
	activeStorage := given.Provider().
		WithUTXOManagement(throughputUTXOManagement()).
		GORMWithCleanDatabase()

	// when:
	response, err := activeStorage.FindOrInsertUser(t.Context(), userIdentityKey)

	// then:
	require.NoError(t, err)
	require.True(t, response.IsNew)

	var baskets []models.OutputBasket
	require.NoError(t, activeStorage.Database.DB.
		Where("user_id = ?", response.User.UserID).
		Order("name").
		Find(&baskets).Error)

	names := make([]string, 0, len(baskets))
	for _, basket := range baskets {
		names = append(names, basket.Name)
	}
	assert.Equal(t, []string{wdk.BasketNameForChange, wdk.BasketNameForFuel, wdk.BasketNameForReserve}, names)

	expectedCfg := throughputUTXOManagement()
	expectedPool := expectedCfg.Throughput.TargetPool()

	for _, basket := range baskets {
		switch basket.Name {
		case wdk.BasketNameForFuel:
			assert.EqualValues(t, 240, basket.MinimumDesiredUTXOValue, "fuel basket min value must be the denomination")
			assert.EqualValues(t, expectedPool, basket.NumberOfDesiredUTXOs)
		case wdk.BasketNameForReserve:
			// The reserve basket's UTXO params are inert (it is never a funding
			// basket); gorm column defaults apply to the zero-valued config, so
			// only existence is asserted here.
		}
	}
}

func TestFindOrInsertUser_PrivacyStrategySeedsOnlyChangeBasket(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	userIdentityKey := "03f17660f611ce531402a2ce1e070380b6fde57aca211d707bfab27bce42d86beb"
	activeStorage := given.Provider().GORMWithCleanDatabase()

	// when:
	response, err := activeStorage.FindOrInsertUser(t.Context(), userIdentityKey)

	// then:
	require.NoError(t, err)

	var baskets []models.OutputBasket
	require.NoError(t, activeStorage.Database.DB.
		Where("user_id = ?", response.User.UserID).
		Find(&baskets).Error)

	require.Len(t, baskets, 1)
	assert.Equal(t, wdk.BasketNameForChange, baskets[0].Name)
}
