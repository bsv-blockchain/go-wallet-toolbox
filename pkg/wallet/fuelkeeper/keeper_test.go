package fuelkeeper_test

import (
	"context"
	"testing"
	"time"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/fuelkeeper"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type fanOutCall struct {
	shape wdk.ShapedChange
}

type fakeWallet struct {
	poolTotal    uint32
	reserveTotal uint32
	fanOuts      []fanOutCall
}

func (f *fakeWallet) ListOutputs(_ context.Context, args sdk.ListOutputsArgs, _ string) (*sdk.ListOutputsResult, error) {
	total := f.poolTotal
	if args.Basket == "reserve" {
		total = f.reserveTotal
	}
	return &sdk.ListOutputsResult{TotalOutputs: total}, nil
}

func (f *fakeWallet) FanOutFuel(_ context.Context, shape wdk.ShapedChange, _ string) (*sdk.CreateActionResult, error) {
	f.fanOuts = append(f.fanOuts, fanOutCall{shape: shape})
	switch string(shape.Basket) {
	case "reserve":
		f.reserveTotal += uint32(shape.Count) //nolint:gosec // test values are small
	case "fuel":
		f.poolTotal += uint32(shape.Count) //nolint:gosec // test values are small
	}
	return &sdk.CreateActionResult{}, nil
}

func keeperConfig() fuelkeeper.Config {
	return fuelkeeper.Config{
		Denomination:         240,
		TargetPoolSize:       1000,
		LowWaterPercent:      60,
		HighWaterPercent:     100,
		FanoutOutputsPerTx:   100,
		FanoutMaxTxsPerRound: 5,
		PoolBasket:           "fuel",
		ReserveBasket:        "reserve",
		Interval:             time.Second,
		ChunkFeeHeadroom:     1000,
		Originator:           "test",
	}
}

func TestRunOnce_PoolHealthyDoesNothing(t *testing.T) {
	fake := &fakeWallet{poolTotal: 700} // above low water (600)
	keeper, err := fuelkeeper.New(fake, keeperConfig(), logging.NewTestLogger(t))
	require.NoError(t, err)

	require.NoError(t, keeper.RunOnce(t.Context()))
	assert.Empty(t, fake.fanOuts)
}

func TestRunOnce_MintsTowardHighWater(t *testing.T) {
	fake := &fakeWallet{poolTotal: 100, reserveTotal: 10}
	keeper, err := fuelkeeper.New(fake, keeperConfig(), logging.NewTestLogger(t))
	require.NoError(t, err)

	// deficit = 1000 - 100 = 900 → ceil(900/100) = 9 leaves, capped at 5 per round
	require.NoError(t, keeper.RunOnce(t.Context()))

	require.Len(t, fake.fanOuts, 5)
	for _, call := range fake.fanOuts {
		assert.Equal(t, "fuel", string(call.shape.Basket))
		assert.EqualValues(t, 100, call.shape.Count)
		assert.EqualValues(t, 240, call.shape.Satoshis)
	}
	assert.EqualValues(t, 600, fake.poolTotal)
}

func TestRunOnce_ChunksReserveFirstWhenEmpty(t *testing.T) {
	fake := &fakeWallet{poolTotal: 0, reserveTotal: 0}
	keeper, err := fuelkeeper.New(fake, keeperConfig(), logging.NewTestLogger(t))
	require.NoError(t, err)

	require.NoError(t, keeper.RunOnce(t.Context()))

	// First call must be the chunk fan-out (interior layer), then the leaves.
	require.NotEmpty(t, fake.fanOuts)
	first := fake.fanOuts[0]
	assert.Equal(t, "reserve", string(first.shape.Basket))
	assert.EqualValues(t, 100*240+1000, first.shape.Satoshis, "chunk value covers a whole leaf + fee headroom")
	assert.EqualValues(t, 5, first.shape.Count, "one chunk per pending leaf")

	leaves := fake.fanOuts[1:]
	require.Len(t, leaves, 5)
	for _, call := range leaves {
		assert.Equal(t, "fuel", string(call.shape.Basket))
	}
}

func TestNew_RejectsInvalidConfig(t *testing.T) {
	mutations := map[string]func(*fuelkeeper.Config){
		"zero denomination":   func(c *fuelkeeper.Config) { c.Denomination = 0 },
		"zero target pool":    func(c *fuelkeeper.Config) { c.TargetPoolSize = 0 },
		"inverted watermarks": func(c *fuelkeeper.Config) { c.LowWaterPercent = 90; c.HighWaterPercent = 50 },
		"zero interval":       func(c *fuelkeeper.Config) { c.Interval = 0 },
		"empty pool basket":   func(c *fuelkeeper.Config) { c.PoolBasket = "" },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			cfg := keeperConfig()
			mutate(&cfg)
			_, err := fuelkeeper.New(&fakeWallet{}, cfg, logging.NewTestLogger(t))
			require.Error(t, err)
		})
	}

	_, err := fuelkeeper.New(nil, keeperConfig(), logging.NewTestLogger(t))
	require.Error(t, err)
}
