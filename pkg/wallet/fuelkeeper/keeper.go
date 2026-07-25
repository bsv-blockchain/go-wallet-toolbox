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
	"sync"
	"sync/atomic"
	"time"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/to"
	"golang.org/x/sync/errgroup"

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

	// StreamLeafCap caps leaf fan-outs per round while SetStreamActive(true):
	// small rounds re-measure inventory often and keep the keeper responsive
	// to target changes, while back-to-back rounds keep minting continuous.
	// 0 → DefaultStreamLeafCap.
	StreamLeafCap uint64
	// StreamYieldMultiple sizes the pause after each fan-out RPC while a
	// stream is active: pause = multiple × RPC duration. With N=3 the keeper
	// consumes ~1/(1+3)=25% of the shared serialized-wallet time, leaving the
	// rest to stream createActions and metrics. 0 → DefaultStreamYieldMultiple.
	StreamYieldMultiple uint64

	// MintConcurrency is how many leaf fan-outs a round runs in parallel.
	// Leaves are independent (each consumes its own reserve chunk), and a
	// wallet whose RPC layer allows concurrent requests can mint several at
	// once — a single serial mint loop cannot refill fast enough to match a
	// high-TPS stream's burn rate. 0 → 1 (serial).
	MintConcurrency uint64
}

// Defaults for the stream-active fairness knobs (see Config).
const (
	DefaultStreamLeafCap       = 10
	DefaultStreamYieldMultiple = 3
	minStreamYield             = 10 * time.Millisecond
	// maxStreamYield caps the post-fan-out pause. The measured RPC duration
	// includes any time spent queued for a shared wallet slot, which inflates
	// under load; an uncapped multiple of it would collapse the keeper's share
	// toward zero and let the pool drain mid-stream.
	maxStreamYield = 3 * time.Second

	// Per-leaf retry for reserve-chunk selection collisions between concurrent
	// leaves (see mintLeaves).
	mintLeafAttempts     = 4
	mintLeafRetryBackoff = 30 * time.Millisecond
)

// FromThroughput derives the keeper configuration from the server-side
// throughput configuration and the resolved denomination.
//
// The denomination MUST match what the server resolves from the same
// throughput config and its fee model (Provider derivation): a mismatch makes
// every leaf shape fail the server's validation and disables minting.
//
// ChunkFeeHeadroom defaults to max(1000, 8 × denomination): the denomination
// scales with the fee rate, so the headroom tracks the leaf transaction's fee
// (≈ outputs × 34 bytes at the same rate) with margin; override it when your
// action shape makes that heuristic wrong.
func FromThroughput(throughput defs.Throughput, denomination uint64) Config {
	headroom := uint64(1000)
	if scaled := 8 * denomination; scaled > headroom {
		headroom = scaled
	}
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
		ChunkFeeHeadroom:     headroom,
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
	logger *slog.Logger

	mu  sync.RWMutex
	cfg Config

	// roundInFlight guards against overlapping rounds (e.g. RunOnce called
	// while Run's ticker round is still minting): both would measure the same
	// deficit and double-mint.
	roundInFlight atomic.Bool

	// streamActive switches minting into fair-share mode: leaf caps per round
	// and a yield after every fan-out RPC so the createAction stream, metrics
	// sampler, and stop paths sharing the serialized wallet are not starved.
	streamActive atomic.Bool
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
	if cfg.StreamLeafCap == 0 {
		cfg.StreamLeafCap = DefaultStreamLeafCap
	}
	if cfg.StreamYieldMultiple == 0 {
		cfg.StreamYieldMultiple = DefaultStreamYieldMultiple
	}
	return &Keeper{
		wallet: walletAPI,
		cfg:    cfg,
		logger: logging.Child(logger, "fuelKeeper"),
	}, nil
}

// SetTargetPoolSize updates the inventory target used for low/high water
// minting. Safe to call while Run is active; the next round observes the new
// value. n must be > 0.
func (k *Keeper) SetTargetPoolSize(n uint64) error {
	if n == 0 {
		return fmt.Errorf("target pool size must be greater than 0")
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	k.cfg.TargetPoolSize = n
	k.logger.Info("fuel keeper target pool updated", slog.Uint64("targetPoolSize", n))
	return nil
}

// TargetPoolSize returns the current inventory target.
func (k *Keeper) TargetPoolSize() uint64 {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.cfg.TargetPoolSize
}

// SetStreamActive toggles fair-share minting. While active, rounds cap leaf
// fan-outs at StreamLeafCap and every fan-out RPC is followed by a
// StreamYieldMultiple×duration pause, bounding the keeper to roughly
// 1/(1+multiple) of the shared serialized-wallet time. Minting never stops —
// catch-up rounds stay back-to-back — it only slows to leave room for the
// stream. Safe to call at any time; a round already in flight observes the
// flag per fan-out.
func (k *Keeper) SetStreamActive(active bool) {
	if k.streamActive.Swap(active) != active {
		k.logger.Info("fuel keeper stream fair-share mode changed", slog.Bool("streamActive", active))
	}
}

// fanOut performs one fan-out RPC and, while a stream is active, pauses
// afterward proportionally to how long the RPC held the shared wallet.
func (k *Keeper) fanOut(ctx context.Context, cfg Config, shape wdk.ShapedChange) error {
	start := time.Now()
	_, err := k.wallet.FanOutFuel(ctx, shape, cfg.Originator)
	k.yieldToStream(ctx, cfg, time.Since(start))
	return err
}

// yieldToStream sleeps multiple×opTook (with a small floor) while a stream is
// active so waiters on the serialized wallet run before the keeper's next RPC.
func (k *Keeper) yieldToStream(ctx context.Context, cfg Config, opTook time.Duration) {
	if !k.streamActive.Load() {
		return
	}
	pause := opTook * time.Duration(cfg.StreamYieldMultiple) //nolint:gosec // multiple is a small config knob
	if pause < minStreamYield {
		pause = minStreamYield
	}
	if pause > maxStreamYield {
		pause = maxStreamYield
	}
	select {
	case <-ctx.Done():
	case <-time.After(pause):
	}
}

// snapshot copies config under the read lock so rounds see a consistent view.
func (k *Keeper) snapshot() Config {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.cfg
}

// catchUpPause is a tiny yield between back-to-back catch-up rounds so the
// process can observe ctx cancel and avoid a pure busy-spin when a round
// no-ops (e.g. another RunOnce already in flight).
const catchUpPause = 10 * time.Millisecond

// Run executes top-up rounds until ctx is canceled. The first round runs
// immediately. While inventory stays below the low-water mark AND rounds keep
// making progress, rounds run back-to-back (only a short yield between them)
// so catch-up is not limited by TopUp.Interval; the configured interval
// applies when the pool is healthy and after unproductive or failed rounds.
func (k *Keeper) Run(ctx context.Context) {
	for {
		catchUp, err := k.runOnce(ctx)
		if err != nil && ctx.Err() == nil {
			k.logger.ErrorContext(ctx, "fuel top-up round failed", logging.Error(err))
		}

		wait := k.snapshot().Interval
		if catchUp {
			wait = catchUpPause
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
	}
}

// RunOnce executes a single top-up round: measure the pool, and when it is
// below low water, mint leaf fan-outs (chunking reserve funds first when the
// reserve is short) toward high water, capped at FanoutMaxTxsPerRound leaves
// and by the chunks actually available. Overlapping rounds are skipped.
func (k *Keeper) RunOnce(ctx context.Context) error {
	_, err := k.runOnce(ctx)
	return err
}

// runOnce is the shared implementation for Run / RunOnce.
//
// catchUp is true only when this round made progress (or was skipped because
// another round is minting) and inventory is still below low water — the
// outer loop then retries after catchUpPause instead of a full interval.
// Rounds that errored or minted nothing (no reserve, no funds, server
// rejecting the shape) return false so retries stay at the configured
// interval; a 10ms retry cadence on a persistent failure would hammer the
// shared wallet with measurement RPCs and flood the logs.
func (k *Keeper) runOnce(ctx context.Context) (catchUp bool, err error) {
	if !k.roundInFlight.CompareAndSwap(false, true) {
		k.logger.DebugContext(ctx, "top-up round already in flight, skipping")
		// Another round is minting; stay in catch-up mode so we retry soon.
		return true, nil
	}
	defer k.roundInFlight.Store(false)

	cfg := k.snapshot()

	inventory, err := k.countBasketOutputs(ctx, cfg.PoolBasket, cfg.Originator)
	if err != nil {
		return false, fmt.Errorf("failed to measure fuel pool: %w", err)
	}

	lowWater := cfg.TargetPoolSize * cfg.LowWaterPercent / 100
	if inventory >= lowWater {
		return false, nil
	}

	fillTarget := cfg.TargetPoolSize * cfg.HighWaterPercent / 100
	deficit := fillTarget - inventory
	leaves := (deficit + cfg.FanoutOutputsPerTx - 1) / cfg.FanoutOutputsPerTx
	if leaves > cfg.FanoutMaxTxsPerRound {
		leaves = cfg.FanoutMaxTxsPerRound
	}
	// Fair-share mode: short rounds so inventory is re-measured often and the
	// keeper reacts to target changes / stream stop quickly. Catch-up keeps
	// rounds back-to-back, so the cap does not stall refill.
	if k.streamActive.Load() && leaves > cfg.StreamLeafCap {
		leaves = cfg.StreamLeafCap
	}

	k.logger.InfoContext(ctx, "fuel pool below low water, minting",
		slog.Uint64("inventory", inventory),
		slog.Uint64("targetPool", cfg.TargetPoolSize),
		slog.Uint64("lowWater", lowWater),
		slog.Uint64("deficit", deficit),
		slog.Uint64("leafTxs", leaves),
		slog.Uint64("maxLeafTxsPerRound", cfg.FanoutMaxTxsPerRound))

	chunks, err := k.ensureChunks(ctx, cfg, leaves)
	if err != nil {
		return false, err
	}
	// Every leaf consumes one chunk; minting past the provisioned chunks
	// would silently drain the default basket via the funding fallback.
	if leaves > chunks {
		k.logger.InfoContext(ctx, "clamping round to available reserve chunks",
			slog.Uint64("leaves", leaves), slog.Uint64("chunks", chunks))
		leaves = chunks
	}

	minted, err := k.mintLeaves(ctx, cfg, leaves)
	if err != nil {
		return false, err
	}

	k.logger.InfoContext(ctx, "fuel top-up round complete",
		slog.Uint64("minted", minted),
		slog.Uint64("inventoryBefore", inventory))

	// Fast follow-up only when this round actually minted and more is needed;
	// a zero-mint round (reserve and default funds exhausted) waits the full
	// interval so the keeper idles gently until the operator deposits.
	stillLow := minted > 0 && inventory+minted < lowWater
	return stillLow, nil
}

// mintLeaves runs up to `leaves` leaf fan-outs with MintConcurrency-bounded
// parallelism and returns how many fuel outputs were minted. While a stream is
// active it stops issuing new leaves past StreamLeafCap (a stream starting
// mid-round shortens even a big idle catch-up round) and each fan-out still
// yields afterward (see fanOut), bounding the keeper's share of the shared
// wallet. On error the started leaves finish and the first error is returned
// with the partial mint count already reflected in the result.
func (k *Keeper) mintLeaves(ctx context.Context, cfg Config, leaves uint64) (uint64, error) {
	conc := cfg.MintConcurrency
	if conc == 0 {
		conc = 1
	}

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(int(conc)) //nolint:gosec // small config knob

	var mintedLeaves atomic.Uint64
	issued := uint64(0)
	for range leaves {
		if gctx.Err() != nil {
			break
		}
		if k.streamActive.Load() && issued >= cfg.StreamLeafCap {
			k.logger.InfoContext(ctx, "stream active, ending mint round at leaf cap",
				slog.Uint64("issuedLeaves", issued))
			break
		}
		issued++
		g.Go(func() error {
			shape := wdk.ShapedChange{
				Count:    cfg.FanoutOutputsPerTx,
				Satoshis: primitives.SatoshiValue(cfg.Denomination),
				Basket:   primitives.StringUnder300(cfg.PoolBasket),
			}
			// Concurrent leaves fund from the same reserve basket and can pick
			// the same chunk; storage reports that as a (deliberately
			// non-retryable) provided-input conflict, which would otherwise
			// fail most of a round. Retry here — a different chunk is chosen on
			// the next attempt — with a short stagger to spread the selection.
			var err error
			for attempt := range mintLeafAttempts {
				if attempt > 0 {
					select {
					case <-gctx.Done():
						return nil
					case <-time.After(time.Duration(attempt) * mintLeafRetryBackoff):
					}
				}
				if err = k.fanOut(gctx, cfg, shape); err == nil {
					mintedLeaves.Add(1)
					return nil
				}
				if gctx.Err() != nil {
					return nil
				}
			}
			return fmt.Errorf("leaf fan-out failed after %d attempts: %w", mintLeafAttempts, err)
		})
	}

	err := g.Wait()
	minted := mintedLeaves.Load() * cfg.FanoutOutputsPerTx
	if err != nil {
		return minted, fmt.Errorf("mint round failed after %d outputs: %w", minted, err)
	}
	return minted, nil
}

// ensureChunks makes sure the reserve basket holds one claimable chunk per
// pending leaf fan-out, splitting default-basket funds into chunks when it
// does not (tree fan-out, interior layer). It returns the number of chunks
// available after provisioning — the caller must not mint more leaves.
func (k *Keeper) ensureChunks(ctx context.Context, cfg Config, leaves uint64) (uint64, error) {
	chunks, err := k.countBasketOutputs(ctx, cfg.ReserveBasket, cfg.Originator)
	if err != nil {
		return 0, fmt.Errorf("failed to measure reserve: %w", err)
	}

	chunkValue := cfg.FanoutOutputsPerTx*cfg.Denomination + cfg.ChunkFeeHeadroom
	for chunks < leaves {
		if ctx.Err() != nil {
			return chunks, fmt.Errorf("chunk provisioning interrupted: %w", ctx.Err())
		}

		needed := leaves - chunks
		if needed > cfg.FanoutOutputsPerTx {
			needed = cfg.FanoutOutputsPerTx
		}
		// The default basket may cover only part of the ask (a full ask is
		// needed × chunkValue satoshis — e.g. 50 × 4000 = 200k). An
		// all-or-nothing fan-out would then mint NOTHING while the stream
		// drains the pool dry, so halve the ask until it funds. Bounded:
		// ~log2(needed) failed attempts before giving up entirely.
		for needed > 1 {
			if err = k.fanOut(ctx, cfg, wdk.ShapedChange{
				Count:    needed,
				Satoshis: primitives.SatoshiValue(chunkValue),
				Basket:   primitives.StringUnder300(cfg.ReserveBasket),
			}); err == nil {
				break
			}
			k.logger.InfoContext(ctx, "chunk fan-out did not fund, halving ask",
				slog.Uint64("count", needed), logging.Error(err))
			needed /= 2
		}
		if needed <= 1 {
			err = k.fanOut(ctx, cfg, wdk.ShapedChange{
				Count:    1,
				Satoshis: primitives.SatoshiValue(chunkValue),
				Basket:   primitives.StringUnder300(cfg.ReserveBasket),
			})
			needed = 1
		}
		if err != nil {
			// Default-basket funds ran out (or another error): mint with what
			// is provisioned so far rather than failing the whole round.
			k.logger.WarnContext(ctx, "chunk fan-out failed, continuing with provisioned chunks",
				slog.Uint64("chunks", chunks), logging.Error(err))
			return chunks, nil
		}
		chunks += needed
	}
	return chunks, nil
}

func (k *Keeper) countBasketOutputs(ctx context.Context, basket, originator string) (uint64, error) {
	result, err := k.wallet.ListOutputs(ctx, sdk.ListOutputsArgs{
		Basket: basket,
		Limit:  to.Ptr(uint32(1)),
	}, originator)
	if err != nil {
		return 0, fmt.Errorf("failed to list %q outputs: %w", basket, err)
	}
	return uint64(result.TotalOutputs), nil
}
