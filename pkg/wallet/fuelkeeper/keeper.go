// Package fuelkeeper keeps the throughput strategy's fuel pool topped up.
//
// The keeper is a CLIENT-side component: fan-out transactions spend the
// operator's outputs and must be signed with the operator's keys, which the
// storage server never holds. Run it in the process that owns the wallet.
//
// Each round it measures the pool (fuel-basket outputs, including immature
// ones — minted-but-unproven fuel counts as inventory so rounds do not
// over-mint), and when the pool is below the low-water mark it mints toward
// the high-water mark: chunk fan-outs split default-basket funds into
// reserve-basket chunks, and leaf fan-outs split chunks into
// exact-denomination fuel. Thresholds and paging on the pool gauges are an
// external concern (see the observability section of the design doc); the
// keeper only replenishes.
package fuelkeeper

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/to"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

// WalletAPI is the narrow wallet surface the keeper drives.
// *wallet.Wallet satisfies it.
type WalletAPI interface {
	ListOutputs(ctx context.Context, args sdk.ListOutputsArgs, originator string) (*sdk.ListOutputsResult, error)
	FanOutFuel(ctx context.Context, shape wdk.ShapedChange, originator string) (*sdk.CreateActionResult, error)
}

// Config sizes the keeper's rounds. FromThroughput derives it from the
// server-side configuration so both sides agree on the shape.
type Config struct {
	Denomination         uint64
	TargetPoolSize       uint64
	LowWaterPercent      uint64
	HighWaterPercent     uint64
	FanoutOutputsPerTx   uint64
	FanoutMaxTxsPerRound uint64
	PoolBasket           string
	ReserveBasket        string
	Interval             time.Duration
	// ChunkFeeHeadroom is added on top of fanout_outputs_per_tx × denomination
	// when sizing chunk outputs, so one chunk always covers a whole leaf
	// fan-out including its fee.
	ChunkFeeHeadroom uint64
	Originator       string
}

// FromThroughput derives the keeper configuration from the server-side
// throughput configuration and the resolved denomination.
func FromThroughput(throughput defs.Throughput, denomination uint64) Config {
	return Config{
		Denomination:         denomination,
		TargetPoolSize:       throughput.TargetPool(),
		LowWaterPercent:      throughput.LowWaterPercent,
		HighWaterPercent:     throughput.HighWaterPercent,
		FanoutOutputsPerTx:   throughput.FanoutOutputsPerTx,
		FanoutMaxTxsPerRound: throughput.FanoutMaxTxsPerRound,
		PoolBasket:           throughput.PoolBasket,
		ReserveBasket:        throughput.ReserveBasket,
		Interval:             throughput.TopUp.Interval(),
		ChunkFeeHeadroom:     1000,
		Originator:           "fuelkeeper",
	}
}

func (c *Config) validate() error {
	if c.Denomination == 0 || c.TargetPoolSize == 0 || c.FanoutOutputsPerTx == 0 {
		return fmt.Errorf("denomination, target pool size, and fanout outputs per tx must be greater than 0")
	}
	if c.LowWaterPercent == 0 || c.LowWaterPercent > c.HighWaterPercent || c.HighWaterPercent > 100 {
		return fmt.Errorf("water marks must satisfy 0 < low (%d) <= high (%d) <= 100", c.LowWaterPercent, c.HighWaterPercent)
	}
	if c.Interval <= 0 {
		return fmt.Errorf("interval must be positive")
	}
	if c.PoolBasket == "" || c.ReserveBasket == "" {
		return fmt.Errorf("pool and reserve basket names must be set")
	}
	return nil
}

// Keeper runs the top-up loop.
type Keeper struct {
	wallet WalletAPI
	cfg    Config
	logger *slog.Logger
}

// New creates a Keeper. It validates the configuration eagerly so
// misconfiguration surfaces at startup, not on the first round.
func New(walletAPI WalletAPI, cfg Config, logger *slog.Logger) (*Keeper, error) {
	if walletAPI == nil {
		return nil, fmt.Errorf("wallet is required")
	}
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid fuel keeper config: %w", err)
	}
	return &Keeper{
		wallet: walletAPI,
		cfg:    cfg,
		logger: logging.Child(logger, "fuelKeeper"),
	}, nil
}

// Run executes rounds on the configured interval until ctx is canceled.
// The first round runs immediately.
func (k *Keeper) Run(ctx context.Context) {
	ticker := time.NewTicker(k.cfg.Interval)
	defer ticker.Stop()

	for {
		if err := k.RunOnce(ctx); err != nil && ctx.Err() == nil {
			k.logger.ErrorContext(ctx, "fuel top-up round failed", logging.Error(err))
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// RunOnce executes a single top-up round: measure the pool, and when it is
// below low water, mint leaf fan-outs (chunking reserve funds first when the
// reserve is empty) toward high water, capped at FanoutMaxTxsPerRound leaves.
func (k *Keeper) RunOnce(ctx context.Context) error {
	inventory, err := k.countBasketOutputs(ctx, k.cfg.PoolBasket)
	if err != nil {
		return fmt.Errorf("failed to measure fuel pool: %w", err)
	}

	lowWater := k.cfg.TargetPoolSize * k.cfg.LowWaterPercent / 100
	if inventory >= lowWater {
		return nil
	}

	fillTarget := k.cfg.TargetPoolSize * k.cfg.HighWaterPercent / 100
	deficit := fillTarget - inventory
	leaves := (deficit + k.cfg.FanoutOutputsPerTx - 1) / k.cfg.FanoutOutputsPerTx
	if leaves > k.cfg.FanoutMaxTxsPerRound {
		leaves = k.cfg.FanoutMaxTxsPerRound
	}

	k.logger.InfoContext(ctx, "fuel pool below low water, minting",
		slog.Uint64("inventory", inventory),
		slog.Uint64("lowWater", lowWater),
		slog.Uint64("deficit", deficit),
		slog.Uint64("leafTxs", leaves))

	if err = k.ensureChunks(ctx, leaves); err != nil {
		return err
	}

	minted := uint64(0)
	for range leaves {
		if ctx.Err() != nil {
			return fmt.Errorf("top-up round interrupted: %w", ctx.Err())
		}
		if _, err = k.wallet.FanOutFuel(ctx, wdk.ShapedChange{
			Count:    k.cfg.FanoutOutputsPerTx,
			Satoshis: primitives.SatoshiValue(k.cfg.Denomination),
			Basket:   primitives.StringUnder300(k.cfg.PoolBasket),
		}, k.cfg.Originator); err != nil {
			return fmt.Errorf("leaf fan-out failed after minting %d outputs: %w", minted, err)
		}
		minted += k.cfg.FanoutOutputsPerTx
	}

	k.logger.InfoContext(ctx, "fuel top-up round complete", slog.Uint64("minted", minted))
	return nil
}

// ensureChunks makes sure the reserve basket holds at least one claimable
// chunk per pending leaf fan-out, splitting default-basket funds into chunks
// when it does not (tree fan-out, interior layer).
func (k *Keeper) ensureChunks(ctx context.Context, leaves uint64) error {
	chunks, err := k.countBasketOutputs(ctx, k.cfg.ReserveBasket)
	if err != nil {
		return fmt.Errorf("failed to measure reserve: %w", err)
	}
	if chunks >= leaves {
		return nil
	}

	needed := leaves - chunks
	if needed > k.cfg.FanoutOutputsPerTx {
		needed = k.cfg.FanoutOutputsPerTx
	}

	chunkValue := k.cfg.FanoutOutputsPerTx*k.cfg.Denomination + k.cfg.ChunkFeeHeadroom
	if _, err = k.wallet.FanOutFuel(ctx, wdk.ShapedChange{
		Count:    needed,
		Satoshis: primitives.SatoshiValue(chunkValue),
		Basket:   primitives.StringUnder300(k.cfg.ReserveBasket),
	}, k.cfg.Originator); err != nil {
		return fmt.Errorf("chunk fan-out failed: %w", err)
	}
	return nil
}

func (k *Keeper) countBasketOutputs(ctx context.Context, basket string) (uint64, error) {
	result, err := k.wallet.ListOutputs(ctx, sdk.ListOutputsArgs{
		Basket: basket,
		Limit:  to.Ptr(uint32(1)),
	}, k.cfg.Originator)
	if err != nil {
		return 0, fmt.Errorf("failed to list %q outputs: %w", basket, err)
	}
	return uint64(result.TotalOutputs), nil
}
