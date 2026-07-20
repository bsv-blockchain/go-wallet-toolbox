package storage_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/satoshi"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/specops"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/randomizer"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

func TestGetBalance_EmptyWallet(t *testing.T) {
	// given:
	ctx := t.Context()
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().GORM()

	// when:
	balance, err := activeStorage.GetBalance(ctx, testusers.Alice.AuthID(), wdk.BasketNameForChange)

	// then:
	require.NoError(t, err)
	assert.Equal(t, uint64(0), balance)
}

func TestGetBalance_AfterTopUp(t *testing.T) {
	// given:
	ctx := t.Context()
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().WithRandomizer(randomizer.NewTestRandomizer()).GORM()

	const topUpSatoshis = uint64(1000)
	user := testusers.Alice
	faucet := given.Faucet(activeStorage, user)
	_, _ = faucet.TopUp(satoshi.MustFrom(topUpSatoshis))

	// when:
	balance, err := activeStorage.GetBalance(ctx, user.AuthID(), wdk.BasketNameForChange)

	// then:
	require.NoError(t, err)
	assert.Equal(t, topUpSatoshis, balance)
}

func TestGetBalance_MultipleTopUps(t *testing.T) {
	// given:
	ctx := t.Context()
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().WithRandomizer(randomizer.NewTestRandomizer()).GORM()

	user := testusers.Alice
	faucet := given.Faucet(activeStorage, user)
	_, _ = faucet.TopUp(satoshi.MustFrom(500))
	_, _ = faucet.TopUp(satoshi.MustFrom(1500))
	_, _ = faucet.TopUp(satoshi.MustFrom(250))

	// when:
	balance, err := activeStorage.GetBalance(ctx, user.AuthID(), "")

	// then: empty basket defaults to change basket; sum of all top-ups
	require.NoError(t, err)
	assert.Equal(t, uint64(2250), balance)
}

func TestGetBalance_AuthFailure(t *testing.T) {
	// given:
	ctx := t.Context()
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().GORM()

	// when: AuthID with nil UserID
	balance, err := activeStorage.GetBalance(ctx, wdk.AuthID{}, wdk.BasketNameForChange)

	// then:
	require.Error(t, err)
	require.ErrorIs(t, err, storage.ErrAuthorization)
	assert.Equal(t, uint64(0), balance)
}

func TestGetBalance_MatchesListOutputsWalletBalanceSpecOp(t *testing.T) {
	// given: fund user then compare dedicated GetBalance with interim #771 ListOutputs spec-op
	ctx := t.Context()
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().WithRandomizer(randomizer.NewTestRandomizer()).GORM()

	const topUpSatoshis = uint64(7777)
	user := testusers.Alice
	faucet := given.Faucet(activeStorage, user)
	_, _ = faucet.TopUp(satoshi.MustFrom(topUpSatoshis))

	// when:
	balance, err := activeStorage.GetBalance(ctx, user.AuthID(), wdk.BasketNameForChange)
	require.NoError(t, err)

	specOpResult, err := activeStorage.ListOutputs(ctx, user.AuthID(), wdk.ListOutputsArgs{
		Basket: primitives.StringUnder300(specops.ListOutputsSpecOpWalletBalance),
		Limit:  1,
	})
	require.NoError(t, err)

	// then: both paths share the same sum helper and must agree
	assert.Equal(t, topUpSatoshis, balance)
	assert.Equal(t, primitives.PositiveInteger(topUpSatoshis), specOpResult.TotalOutputs)
	assert.Empty(t, specOpResult.Outputs)
}
