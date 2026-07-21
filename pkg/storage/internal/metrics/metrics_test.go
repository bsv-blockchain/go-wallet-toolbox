package metrics_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/metrics"
)

func collect(t *testing.T, reader *sdkmetric.ManualReader) map[string]metricdata.Metrics {
	t.Helper()
	var data metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(t.Context(), &data))

	byName := map[string]metricdata.Metrics{}
	for _, scope := range data.ScopeMetrics {
		for _, m := range scope.Metrics {
			byName[m.Name] = m
		}
	}
	return byName
}

func TestFunderCountersAndPoolGauges(t *testing.T) {
	// given: a manual-reader meter provider installed globally
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	prev := otel.GetMeterProvider()
	otel.SetMeterProvider(provider)
	t.Cleanup(func() { otel.SetMeterProvider(prev) })

	// and: pool gauges registered over a fake snapshot
	unregister, err := metrics.RegisterPoolGauges(metrics.PoolGaugeConfig{
		PoolBasket:        "fuel",
		ReserveBasket:     "reserve",
		Denomination:      240,
		TargetTPS:         10,
		FanoutFeeOverhead: 0.15,
	}, func(_ context.Context) ([]metrics.PoolRow, error) {
		return []metrics.PoolRow{
			{Basket: "fuel", Status: "mined", Count: 100, Satoshis: 24_000},
			{Basket: "fuel", Status: "unproven", Count: 20, Satoshis: 4_800},
			{Basket: "reserve", Status: "mined", Count: 2, Satoshis: 50_000},
		}, nil
	})
	require.NoError(t, err)
	t.Cleanup(unregister)

	// when: funding outcomes recorded
	metrics.RecordFundingOutcome(t.Context(), "exact_match")
	metrics.RecordFundingOutcome(t.Context(), "exact_match")
	metrics.RecordFundingOutcome(t.Context(), "fallback")
	metrics.RecordNotEnoughFunds(t.Context())
	metrics.RecordContentionRetry(t.Context())

	byName := collect(t, reader)

	// then: counters carry the recorded values
	claims, ok := byName["wallet.funder.claims"].Data.(metricdata.Sum[int64])
	require.True(t, ok)
	var total int64
	for _, dp := range claims.DataPoints {
		total += dp.Value
	}
	assert.EqualValues(t, 3, total)

	nef, ok := byName["wallet.funder.not_enough_funds"].Data.(metricdata.Sum[int64])
	require.True(t, ok)
	require.Len(t, nef.DataPoints, 1)
	assert.EqualValues(t, 1, nef.DataPoints[0].Value)

	// and: gauges report inventory and runway
	spendable, ok := byName["wallet.utxo.pool.spendable"].Data.(metricdata.Gauge[int64])
	require.True(t, ok)
	assert.Len(t, spendable.DataPoints, 3)

	poolRunway, ok := byName["wallet.utxo.pool.runway_seconds"].Data.(metricdata.Gauge[float64])
	require.True(t, ok)
	require.Len(t, poolRunway.DataPoints, 1)
	assert.InDelta(t, 12.0, poolRunway.DataPoints[0].Value, 0.001, "120 fuel UTXOs / 10 tps")

	reserveBalance, ok := byName["wallet.utxo.reserve.balance_satoshis"].Data.(metricdata.Gauge[int64])
	require.True(t, ok)
	require.Len(t, reserveBalance.DataPoints, 1)
	assert.EqualValues(t, 50_000, reserveBalance.DataPoints[0].Value)

	reserveRunway, ok := byName["wallet.utxo.reserve.runway_seconds"].Data.(metricdata.Gauge[float64])
	require.True(t, ok)
	require.Len(t, reserveRunway.DataPoints, 1)
	// 50_000 sats / (240 × 1.15 × 10 per second) ≈ 18.12 s
	assert.InDelta(t, 50_000.0/(240*1.15*10), reserveRunway.DataPoints[0].Value, 0.01)
}
