package metrics_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/metrics"
)

func TestServiceCallMetrics(t *testing.T) {
	// when:
	metrics.RecordServiceCall(t.Context(), "merklePath", "arcade", "ok")
	metrics.RecordServiceCall(t.Context(), "merklePath", "arcade", "not_found")
	metrics.RecordServiceCall(t.Context(), "merklePath", "arcade", "not_found")
	metrics.RecordBumpLeafMissing(t.Context())

	byName := collect(t)

	// then: one series per outcome
	calls, ok := byName["wallet.services.calls_total"].Data.(metricdata.Sum[int64])
	require.True(t, ok)
	byOutcome := map[string]int64{}
	for _, dp := range calls.DataPoints {
		outcome, _ := dp.Attributes.Value("outcome")
		byOutcome[outcome.AsString()] = dp.Value
	}
	assert.Equal(t, map[string]int64{"ok": 1, "not_found": 2}, byOutcome)

	// and:
	leaf, ok := byName["wallet.beef.bump_leaf_missing_total"].Data.(metricdata.Sum[int64])
	require.True(t, ok)
	require.Len(t, leaf.DataPoints, 1)
	assert.EqualValues(t, 1, leaf.DataPoints[0].Value)
}
