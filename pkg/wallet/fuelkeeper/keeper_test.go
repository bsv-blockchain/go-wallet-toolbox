package fuelkeeper_test

import (
	"context"
	"errors"
	"sync"
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
	mu           sync.Mutex
	poolTotal    uint32
	reserveTotal uint32
	fanOuts      []fanOutCall
	// fanOutDelay simulates the wallet RPC holding the shared client.
	fanOutDelay time.Duration
	// maxChunkCount, when > 0, fails reserve fan-outs asking for more than
	// this many chunks — simulates a default basket that cannot fund the
	// full ask ("not enough funds").
	maxChunkCount uint64
}

func (f *fakeWallet) ListOutputs(_ context.Context, args sdk.ListOutputsArgs, _ string) (*sdk.ListOutputsResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	total := f.poolTotal
	if args.Basket == "reserve" {
		total = f.reserveTotal
	}
	return &sdk.ListOutputsResult{TotalOutputs: total}, nil
}

func (f *fakeWallet) FanOutFuel(_ context.Context, shape wdk.ShapedChange, _ string) (*sdk.CreateActionResult, error) {
	f.mu.Lock()
	if string(shape.Basket) == "reserve" && f.maxChunkCount > 0 && uint64(shape.Count) > f.maxChunkCount {
		f.mu.Unlock()
		return nil, errors.New("funding failed: not enough funds")
	}
	f.fanOuts = append(f.fanOuts, fanOutCall{shape: shape})
	switch string(shape.Basket) {
	case "reserve":
		f.reserveTotal += uint32(shape.Count) //nolint:gosec // test values are small
	case "fuel":
		f.poolTotal += uint32(shape.Count) //nolint:gosec // test values are small
	}
	delay := f.fanOutDelay
	f.mu.Unlock()
	if delay > 0 {
		time.Sleep(delay)
	}
	return &sdk.CreateActionResult{}, nil
}

func (f *fakeWallet) pool() uint32 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.poolTotal
}

// fuelFanOuts counts recorded leaf fan-outs into the fuel basket.
func (f *fakeWallet) fuelFanOuts() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.fanOuts {
		if string(c.shape.Basket) == "fuel" {
			n++
		}
	}
	return n
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

func TestSetTargetPoolSize_UsedOnNextRound(t *testing.T) {
	fake := &fakeWallet{poolTotal: 700, reserveTotal: 20} // above low water at target 1000 (600)
	keeper, err := fuelkeeper.New(fake, keeperConfig(), logging.NewTestLogger(t))
	require.NoError(t, err)

	require.NoError(t, keeper.RunOnce(t.Context()))
	assert.Empty(t, fake.fanOuts, "healthy at target 1000")

	// Raise target so 700 is below the new low water (60% of 2000 = 1200).
	require.NoError(t, keeper.SetTargetPoolSize(2000))
	require.Equal(t, uint64(2000), keeper.TargetPoolSize())

	require.NoError(t, keeper.RunOnce(t.Context()))
	require.NotEmpty(t, fake.fanOuts, "should mint after target raised past inventory")
}

func TestRun_CatchUpLoopsWhileBelowLowWater(t *testing.T) {
	// Cap leaves per round so filling high water needs multiple rounds; Run must
	// not wait the full interval between them while still below low water.
	cfg := keeperConfig()
	cfg.TargetPoolSize = 1000
	cfg.FanoutMaxTxsPerRound = 2 // 200 fuel/round; high water 1000 needs ≥5 rounds
	cfg.Interval = time.Hour     // would stall the test if catch-up waited on interval
	fake := &fakeWallet{poolTotal: 0, reserveTotal: 100}
	keeper, err := fuelkeeper.New(fake, cfg, logging.NewTestLogger(t))
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	done := make(chan struct{})
	go func() {
		keeper.Run(ctx)
		close(done)
	}()

	require.Eventually(t, func() bool {
		return fake.pool() >= 600 // low water
	}, 2*time.Second, 10*time.Millisecond, "catch-up should mint past low water without hourly waits")

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not exit after cancel")
	}
}

func TestRunOnce_StreamActiveCapsLeavesPerRound(t *testing.T) {
	cfg := keeperConfig()
	cfg.TargetPoolSize = 10_000 // deficit needs 100 leaves; round max is 5
	cfg.StreamLeafCap = 2
	cfg.StreamYieldMultiple = 1
	fake := &fakeWallet{poolTotal: 0, reserveTotal: 100} // chunks pre-provisioned
	keeper, err := fuelkeeper.New(fake, cfg, logging.NewTestLogger(t))
	require.NoError(t, err)

	// Idle: full round (FanoutMaxTxsPerRound = 5 leaves).
	require.NoError(t, keeper.RunOnce(t.Context()))
	require.Equal(t, 5, fake.fuelFanOuts())

	// Stream active: rounds shrink to StreamLeafCap.
	keeper.SetStreamActive(true)
	require.NoError(t, keeper.RunOnce(t.Context()))
	require.Equal(t, 5+2, fake.fuelFanOuts())

	// Back to idle: full rounds again.
	keeper.SetStreamActive(false)
	require.NoError(t, keeper.RunOnce(t.Context()))
	require.Equal(t, 5+2+5, fake.fuelFanOuts())
}

func TestRunOnce_StreamActiveYieldsAfterEachFanOut(t *testing.T) {
	cfg := keeperConfig()
	cfg.TargetPoolSize = 10_000
	cfg.StreamLeafCap = 3
	cfg.StreamYieldMultiple = 3
	fake := &fakeWallet{poolTotal: 0, reserveTotal: 100, fanOutDelay: 20 * time.Millisecond}
	keeper, err := fuelkeeper.New(fake, cfg, logging.NewTestLogger(t))
	require.NoError(t, err)
	keeper.SetStreamActive(true)

	start := time.Now()
	require.NoError(t, keeper.RunOnce(t.Context()))
	elapsed := time.Since(start)

	require.Equal(t, 3, fake.fuelFanOuts())
	// 3 fan-outs × 20ms each + 3 yields × ≥60ms (3× op time) ⇒ ≥240ms.
	// Lower-bound only: sleeps guarantee the minimum, jitter can only add.
	require.GreaterOrEqual(t, elapsed, 200*time.Millisecond,
		"stream-active round must pause after each fan-out to share the wallet")
}

func TestRunOnce_StreamYieldRespectsCancel(t *testing.T) {
	cfg := keeperConfig()
	cfg.TargetPoolSize = 10_000
	cfg.StreamLeafCap = 5
	cfg.StreamYieldMultiple = 1000 // yields would take ~minutes if not canceled
	fake := &fakeWallet{poolTotal: 0, reserveTotal: 100, fanOutDelay: 10 * time.Millisecond}
	keeper, err := fuelkeeper.New(fake, cfg, logging.NewTestLogger(t))
	require.NoError(t, err)
	keeper.SetStreamActive(true)

	ctx, cancel := context.WithCancel(t.Context())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	start := time.Now()
	_ = keeper.RunOnce(ctx) // interruption error is fine; must not hang
	require.Less(t, time.Since(start), 5*time.Second)
}

func TestRunOnce_PartialFundsStillProvisionChunks(t *testing.T) {
	// The default basket can only fund small chunk fan-outs: the keeper must
	// halve its ask and keep minting instead of failing the round with zero
	// chunks while the stream drains the pool (observed live: 50-chunk ask =
	// 200k sats vs 49.5k available → "not enough funds" → no mint at all).
	cfg := keeperConfig() // FanoutMaxTxsPerRound 5 → wants 5 chunks
	fake := &fakeWallet{poolTotal: 0, reserveTotal: 0, maxChunkCount: 3}
	keeper, err := fuelkeeper.New(fake, cfg, logging.NewTestLogger(t))
	require.NoError(t, err)

	require.NoError(t, keeper.RunOnce(t.Context()))

	// Ask 5 fails, halved 2 funds, then 3 funds → 5 chunks → 5 leaves minted.
	require.EqualValues(t, 5, fake.reserveTotal, "chunks provisioned despite partial funds")
	require.Equal(t, 5, fake.fuelFanOuts())
	require.EqualValues(t, 500, fake.pool())
}

func TestRunOnce_NoFundsAtAllMintsNothingWithoutError(t *testing.T) {
	cfg := keeperConfig()
	// Even a single-chunk ask fails: round must degrade gracefully (no error,
	// no mint) and Run retries on the configured interval.
	fake := &fakeWallet{poolTotal: 0, reserveTotal: 0, maxChunkCount: 0}
	// maxChunkCount 0 means unlimited in the fake; use a rejecting wrapper.
	rejecting := &rejectingReserveWallet{fakeWallet: fake}
	keeper, err := fuelkeeper.New(rejecting, cfg, logging.NewTestLogger(t))
	require.NoError(t, err)

	require.NoError(t, keeper.RunOnce(t.Context()))
	require.Equal(t, 0, fake.fuelFanOuts())
}

// rejectingReserveWallet fails every reserve fan-out ("wallet empty").
type rejectingReserveWallet struct {
	*fakeWallet
}

func (r *rejectingReserveWallet) FanOutFuel(ctx context.Context, shape wdk.ShapedChange, o string) (*sdk.CreateActionResult, error) {
	if string(shape.Basket) == "reserve" {
		return nil, errors.New("funding failed: not enough funds")
	}
	return r.fakeWallet.FanOutFuel(ctx, shape, o)
}

func TestSetTargetPoolSize_RejectsZero(t *testing.T) {
	keeper, err := fuelkeeper.New(&fakeWallet{}, keeperConfig(), logging.NewTestLogger(t))
	require.NoError(t, err)
	require.Error(t, keeper.SetTargetPoolSize(0))
	require.Equal(t, uint64(1000), keeper.TargetPoolSize())
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
