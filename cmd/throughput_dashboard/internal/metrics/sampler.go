// Package metrics samples wallet baskets and stream stats for the dashboard.
package metrics

import (
	"context"
	"log/slog"
	"sync"
	"time"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/to"

	"github.com/bsv-blockchain/go-wallet-toolbox/cmd/throughput_dashboard/internal/stream"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// Event is a dashboard SSE-friendly event.
type Event struct {
	Type      string         `json:"type"`
	Timestamp string         `json:"timestamp"`
	Payload   map[string]any `json:"payload"`
}

// Tick is a periodic sample of stream + fuel gauges.
type Tick struct {
	Timestamp      string       `json:"timestamp"`
	Stream         stream.Stats `json:"stream"`
	TPSSucceeded   uint64       `json:"tps_succeeded"`
	TPSFailed      uint64       `json:"tps_failed"`
	TPSAttempted   uint64       `json:"tps_attempted"`
	DefaultSats    uint64       `json:"default_sats"`
	FuelCount      uint64       `json:"fuel_count"`
	ReserveCount   uint64       `json:"reserve_count"`
	FuelRunwaySec  float64      `json:"fuel_runway_seconds"`
	TargetTPS      uint64       `json:"target_tps"`
	Denomination   uint64       `json:"denomination"`
	LowWater       uint64       `json:"low_water"`
	HighWater      uint64       `json:"high_water"`
	TargetPoolSize uint64       `json:"target_pool_size"`
}

// WalletAPI is the wallet surface used by the sampler.
type WalletAPI interface {
	Balance(ctx context.Context) (uint64, error)
	ListOutputs(ctx context.Context, args sdk.ListOutputsArgs, originator string) (*sdk.ListOutputsResult, error)
}

// Sampler polls balances and stream counters and records top-up inventory deltas.
type Sampler struct {
	wallet       WalletAPI
	ctrl         *stream.Controller
	originator   string
	logger       *slog.Logger
	interval     time.Duration
	targetTPS    uint64
	denomination uint64
	targetPool   uint64
	lowWater     uint64
	highWater    uint64

	mu            sync.RWMutex
	lastTick      Tick
	events        []Event
	maxEvents     int
	prevAttempted uint64
	prevSucceeded uint64
	prevFailed    uint64
	prevFuel      uint64
	prevReserve   uint64
	havePrev      bool

	subscribersMu sync.Mutex
	subscribers   map[chan Event]struct{}
}

// Config configures a metrics Sampler.
type Config struct {
	Originator       string
	Interval         time.Duration
	TargetTPS        uint64
	Denomination     uint64
	TargetPool       uint64
	LowWaterPercent  uint64
	HighWaterPercent uint64
	Logger           *slog.Logger
}

// NewSampler builds a metrics sampler.
func NewSampler(wallet WalletAPI, ctrl *stream.Controller, cfg Config) *Sampler {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	interval := cfg.Interval
	if interval <= 0 {
		interval = time.Second
	}
	low := cfg.TargetPool * cfg.LowWaterPercent / 100
	high := cfg.TargetPool * cfg.HighWaterPercent / 100
	return &Sampler{
		wallet:       wallet,
		ctrl:         ctrl,
		originator:   cfg.Originator,
		logger:       logger,
		interval:     interval,
		targetTPS:    cfg.TargetTPS,
		denomination: cfg.Denomination,
		targetPool:   cfg.TargetPool,
		lowWater:     low,
		highWater:    high,
		maxEvents:    200,
		subscribers:  make(map[chan Event]struct{}),
	}
}

// Run samples until ctx is cancelled.
func (s *Sampler) Run(ctx context.Context) {
	// Immediate sample then tick.
	s.sample(ctx)
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.sample(ctx)
		}
	}
}

// LastTick returns the most recent sample.
func (s *Sampler) LastTick() Tick {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastTick
}

// RecentEvents returns a copy of the event ring.
func (s *Sampler) RecentEvents() []Event {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Event, len(s.events))
	copy(out, s.events)
	return out
}

// Subscribe receives live events. Caller must Unsubscribe.
func (s *Sampler) Subscribe() chan Event {
	ch := make(chan Event, 32)
	s.subscribersMu.Lock()
	s.subscribers[ch] = struct{}{}
	s.subscribersMu.Unlock()
	return ch
}

// Unsubscribe removes a subscriber and closes its channel.
func (s *Sampler) Unsubscribe(ch chan Event) {
	s.subscribersMu.Lock()
	if _, ok := s.subscribers[ch]; ok {
		delete(s.subscribers, ch)
		close(ch)
	}
	s.subscribersMu.Unlock()
}

func (s *Sampler) sample(ctx context.Context) {
	now := time.Now().UTC()
	stats, dAtt, dSucc, dFail := s.ctrl.SnapshotAndDelta(s.prevAttempted, s.prevSucceeded, s.prevFailed)
	s.prevAttempted = stats.Attempted
	s.prevSucceeded = stats.Succeeded
	s.prevFailed = stats.Failed

	// Bound each storage call so a hung RPC cannot freeze ticks forever.
	const callTimeout = 15 * time.Second

	defaultSats, err := s.withTimeout(ctx, callTimeout, func(cctx context.Context) (uint64, error) {
		return s.wallet.Balance(cctx)
	})
	if err != nil {
		s.logger.Warn("balance sample failed", "error", err)
	}
	fuelCount, err := s.withTimeout(ctx, callTimeout, func(cctx context.Context) (uint64, error) {
		return s.basketCount(cctx, wdk.BasketNameForFuel)
	})
	if err != nil {
		s.logger.Warn("fuel count sample failed", "error", err)
	}
	reserveCount, err := s.withTimeout(ctx, callTimeout, func(cctx context.Context) (uint64, error) {
		return s.basketCount(cctx, wdk.BasketNameForReserve)
	})
	if err != nil {
		s.logger.Warn("reserve count sample failed", "error", err)
	}

	var runway float64
	if s.targetTPS > 0 {
		runway = float64(fuelCount) / float64(s.targetTPS)
	}

	tick := Tick{
		Timestamp:      now.Format(time.RFC3339Nano),
		Stream:         stats,
		TPSSucceeded:   dSucc,
		TPSFailed:      dFail,
		TPSAttempted:   dAtt,
		DefaultSats:    defaultSats,
		FuelCount:      fuelCount,
		ReserveCount:   reserveCount,
		FuelRunwaySec:  runway,
		TargetTPS:      s.targetTPS,
		Denomination:   s.denomination,
		LowWater:       s.lowWater,
		HighWater:      s.highWater,
		TargetPoolSize: s.targetPool,
	}

	// Detect top-up activity via inventory increases.
	if s.havePrev {
		if fuelCount > s.prevFuel {
			s.emit(Event{
				Type:      "topup",
				Timestamp: tick.Timestamp,
				Payload: map[string]any{
					"basket":  wdk.BasketNameForFuel,
					"before":  s.prevFuel,
					"after":   fuelCount,
					"delta":   fuelCount - s.prevFuel,
					"kind":    "fuel_inventory_increase",
					"message": "fuel pool inventory increased (FuelKeeper mint activity)",
				},
			})
		}
		if reserveCount > s.prevReserve {
			s.emit(Event{
				Type:      "topup",
				Timestamp: tick.Timestamp,
				Payload: map[string]any{
					"basket":  wdk.BasketNameForReserve,
					"before":  s.prevReserve,
					"after":   reserveCount,
					"delta":   reserveCount - s.prevReserve,
					"kind":    "reserve_inventory_increase",
					"message": "reserve basket inventory increased (chunk fan-out)",
				},
			})
		}
	}
	s.prevFuel = fuelCount
	s.prevReserve = reserveCount
	s.havePrev = true

	s.mu.Lock()
	s.lastTick = tick
	s.mu.Unlock()

	s.emit(Event{
		Type:      "tick",
		Timestamp: tick.Timestamp,
		Payload: map[string]any{
			"tick": tick,
		},
	})
}

func (s *Sampler) basketCount(ctx context.Context, basket string) (uint64, error) {
	result, err := s.wallet.ListOutputs(ctx, sdk.ListOutputsArgs{
		Basket: basket,
		Limit:  to.Ptr(uint32(1)),
	}, s.originator)
	if err != nil {
		return 0, err
	}
	return uint64(result.TotalOutputs), nil
}

func (s *Sampler) withTimeout(ctx context.Context, d time.Duration, fn func(context.Context) (uint64, error)) (uint64, error) {
	cctx, cancel := context.WithTimeout(ctx, d)
	defer cancel()
	return fn(cctx)
}

func (s *Sampler) emit(ev Event) {
	s.mu.Lock()
	s.events = append(s.events, ev)
	if len(s.events) > s.maxEvents {
		s.events = s.events[len(s.events)-s.maxEvents:]
	}
	s.mu.Unlock()

	s.subscribersMu.Lock()
	defer s.subscribersMu.Unlock()
	for ch := range s.subscribers {
		select {
		case ch <- ev:
		default:
			// drop if slow subscriber
		}
	}
}
