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

// RegisterPoolGauges registers the pool/reserve observable gauges backed by
// snapshot. It returns an unregister func. With no MeterProvider set the
// callback never fires.
func RegisterPoolGauges(cfg PoolGaugeConfig, snapshot PoolSnapshotFunc) (func(), error) {
	meter := otel.Meter(meterName)

	spendable, err := meter.Int64ObservableGauge("wallet.utxo.pool.spendable",
		metric.WithDescription("Not-reserved UTXOs by basket and status"))
	if err != nil {
		return nil, fmt.Errorf("failed to create pool.spendable gauge: %w", err)
	}
	poolRunway, err := meter.Float64ObservableGauge("wallet.utxo.pool.runway_seconds",
		metric.WithDescription("Seconds until fuel pool exhaustion at the rated claim load"))
	if err != nil {
		return nil, fmt.Errorf("failed to create pool.runway_seconds gauge: %w", err)
	}
	reserveBalance, err := meter.Int64ObservableGauge("wallet.utxo.reserve.balance_satoshis",
		metric.WithDescription("Satoshis held in the reserve basket"))
	if err != nil {
		return nil, fmt.Errorf("failed to create reserve.balance_satoshis gauge: %w", err)
	}
	reserveRunway, err := meter.Float64ObservableGauge("wallet.utxo.reserve.runway_seconds",
		metric.WithDescription("Seconds until reserve exhaustion at the rated burn (incl. fan-out fee overhead)"))
	if err != nil {
		return nil, fmt.Errorf("failed to create reserve.runway_seconds gauge: %w", err)
	}

	registration, err := meter.RegisterCallback(func(ctx context.Context, observer metric.Observer) error {
		rows, snapErr := snapshot(ctx)
		if snapErr != nil {
			return fmt.Errorf("pool snapshot failed: %w", snapErr)
		}

		var poolCount int64
		var reserveSats int64
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

		observer.ObserveInt64(reserveBalance, reserveSats)
		if cfg.TargetTPS > 0 {
			observer.ObserveFloat64(poolRunway, float64(poolCount)/float64(cfg.TargetTPS))
			burnPerSecond := float64(cfg.Denomination) * (1 + cfg.FanoutFeeOverhead) * float64(cfg.TargetTPS)
			if burnPerSecond > 0 {
				observer.ObserveFloat64(reserveRunway, float64(reserveSats)/burnPerSecond)
			}
		}
		return nil
	}, spendable, poolRunway, reserveBalance, reserveRunway)
	if err != nil {
		return nil, fmt.Errorf("failed to register pool gauge callback: %w", err)
	}

	return func() { _ = registration.Unregister() }, nil
}
