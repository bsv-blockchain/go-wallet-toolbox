// Package metrics defines the storage-side OpenTelemetry instruments of the
// throughput UTXO-management strategy (proposal §5.4). All instruments are
// registered against the global meter: they are no-ops until the process
// enables a MeterProvider (tracing.EnableMetrics), so the privacy strategy and
// unconfigured deployments pay nothing. Thresholds and paging are an external
// concern — the wallet only emits telemetry.
package metrics

import (
	"context"
	"fmt"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const meterName = "github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"

var (
	initOnce sync.Once

	funderClaims         metric.Int64Counter
	funderNotEnoughFunds metric.Int64Counter
	contentionRetries    metric.Int64Counter
)

func ensureInstruments() {
	initOnce.Do(func() {
		meter := otel.Meter(meterName)
		// Instrument creation only fails on invalid names; fall back to no-op
		// instruments rather than propagating an error into the hot path.
		funderClaims, _ = meter.Int64Counter("wallet.funder.claims",
			metric.WithDescription("Funding requests by outcome (exact_match | multi_claim | fallback)"))
		funderNotEnoughFunds, _ = meter.Int64Counter("wallet.funder.not_enough_funds",
			metric.WithDescription("Funding failures surfaced as not-enough-funds"))
		contentionRetries, _ = meter.Int64Counter("wallet.funder.contention_retries",
			metric.WithDescription("Funding transaction retries after UTXO contention"))
	})
}

// RecordFundingOutcome counts one funded request by outcome.
func RecordFundingOutcome(ctx context.Context, outcome string) {
	ensureInstruments()
	if funderClaims != nil {
		funderClaims.Add(ctx, 1, metric.WithAttributes(attribute.String("result", outcome)))
	}
}

// RecordNotEnoughFunds counts one funding failure.
func RecordNotEnoughFunds(ctx context.Context) {
	ensureInstruments()
	if funderNotEnoughFunds != nil {
		funderNotEnoughFunds.Add(ctx, 1)
	}
}

// RecordContentionRetry counts one contention-triggered funding retry.
func RecordContentionRetry(ctx context.Context) {
	ensureInstruments()
	if contentionRetries != nil {
		contentionRetries.Add(ctx, 1)
	}
}

// PoolRow is one (basket, status) bucket of the not-reserved UTXO inventory.
type PoolRow struct {
	Basket   string
	Status   string
	Count    int64
	Satoshis int64
}

// PoolSnapshotFunc returns the current not-reserved UTXO inventory grouped by
// basket and status. It runs once per metrics export interval, never on the
// funding hot path.
type PoolSnapshotFunc func(ctx context.Context) ([]PoolRow, error)

// PoolGaugeConfig carries the throughput parameters runway is derived from.
type PoolGaugeConfig struct {
	PoolBasket    string
	ReserveBasket string
	Denomination  uint64
	TargetTPS     uint64
	// FanoutFeeOverhead is the fan-out fee cost as a fraction of fuel value
	// (~0.15 at a 24-sat denomination; see proposal §5.3/§6) — reserve runway
	// understates burn without it.
	FanoutFeeOverhead float64
}

type poolGauges struct {
	spendable      metric.Int64ObservableGauge
	poolRunway     metric.Float64ObservableGauge
	reserveBalance metric.Int64ObservableGauge
	reserveRunway  metric.Float64ObservableGauge
}

// RegisterPoolGauges registers the pool/reserve observable gauges backed by
// snapshot. It returns an unregister func. With no MeterProvider set the
// callback never fires.
func RegisterPoolGauges(cfg PoolGaugeConfig, snapshot PoolSnapshotFunc) (func(), error) {
	meter := otel.Meter(meterName)
	gauges, err := newPoolGauges(meter)
	if err != nil {
		return nil, err
	}

	registration, err := meter.RegisterCallback(func(ctx context.Context, observer metric.Observer) error {
		return observePoolSnapshot(ctx, observer, cfg, gauges, snapshot)
	}, gauges.spendable, gauges.poolRunway, gauges.reserveBalance, gauges.reserveRunway)
	if err != nil {
		return nil, fmt.Errorf("failed to register pool gauge callback: %w", err)
	}

	return func() { _ = registration.Unregister() }, nil
}

func newPoolGauges(meter metric.Meter) (poolGauges, error) {
	var gauges poolGauges
	var err error

	gauges.spendable, err = meter.Int64ObservableGauge("wallet.utxo.pool.spendable",
		metric.WithDescription("Not-reserved UTXOs by basket and status"))
	if err != nil {
		return poolGauges{}, fmt.Errorf("failed to create pool.spendable gauge: %w", err)
	}
	gauges.poolRunway, err = meter.Float64ObservableGauge("wallet.utxo.pool.runway_seconds",
		metric.WithDescription("Seconds until fuel pool exhaustion at the rated claim load"))
	if err != nil {
		return poolGauges{}, fmt.Errorf("failed to create pool.runway_seconds gauge: %w", err)
	}
	gauges.reserveBalance, err = meter.Int64ObservableGauge("wallet.utxo.reserve.balance_satoshis",
		metric.WithDescription("Satoshis held in the reserve basket"))
	if err != nil {
		return poolGauges{}, fmt.Errorf("failed to create reserve.balance_satoshis gauge: %w", err)
	}
	gauges.reserveRunway, err = meter.Float64ObservableGauge("wallet.utxo.reserve.runway_seconds",
		metric.WithDescription("Seconds until reserve exhaustion at the rated burn (incl. fan-out fee overhead)"))
	if err != nil {
		return poolGauges{}, fmt.Errorf("failed to create reserve.runway_seconds gauge: %w", err)
	}
	return gauges, nil
}

func observePoolSnapshot(ctx context.Context, observer metric.Observer, cfg PoolGaugeConfig, gauges poolGauges, snapshot PoolSnapshotFunc) error {
	rows, err := snapshot(ctx)
	if err != nil {
		return fmt.Errorf("pool snapshot failed: %w", err)
	}

	poolCount, reserveSats := observeSpendableRows(observer, gauges.spendable, cfg, rows)
	observer.ObserveInt64(gauges.reserveBalance, reserveSats)
	observeRunways(observer, gauges, cfg, poolCount, reserveSats)
	return nil
}

func observeSpendableRows(observer metric.Observer, spendable metric.Int64ObservableGauge, cfg PoolGaugeConfig, rows []PoolRow) (poolCount, reserveSats int64) {
	for _, row := range rows {
		observer.ObserveInt64(spendable, row.Count, metric.WithAttributes(
			attribute.String("basket", row.Basket),
			attribute.String("status", row.Status),
		))
		if row.Basket == cfg.PoolBasket {
			poolCount += row.Count
		}
		if row.Basket == cfg.ReserveBasket {
			reserveSats += row.Satoshis
		}
	}
	return poolCount, reserveSats
}

func observeRunways(observer metric.Observer, gauges poolGauges, cfg PoolGaugeConfig, poolCount, reserveSats int64) {
	if cfg.TargetTPS == 0 {
		return
	}
	observer.ObserveFloat64(gauges.poolRunway, float64(poolCount)/float64(cfg.TargetTPS))
	burnPerSecond := float64(cfg.Denomination) * (1 + cfg.FanoutFeeOverhead) * float64(cfg.TargetTPS)
	if burnPerSecond > 0 {
		observer.ObserveFloat64(gauges.reserveRunway, float64(reserveSats)/burnPerSecond)
	}
}
