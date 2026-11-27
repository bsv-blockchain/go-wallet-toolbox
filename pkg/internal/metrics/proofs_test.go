package metrics_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/metrics"
)

func TestProofMetrics(t *testing.T) {
	// when:
	metrics.RecordOverdueProofTxs(t.Context(), 3)
	metrics.RecordManualProofCheck(t.Context(), metrics.ProofCheckNotFound)
	metrics.RecordProofApplied(t.Context(), metrics.ProofSourceSSE)

	// then:
	byName := collect(t)

	gauge, ok := byName["wallet.proofs.overdue_txs"].Data.(metricdata.Gauge[int64])
	require.True(t, ok)
	require.Equal(t, int64(3), gauge.DataPoints[0].Value)

	checks, ok := byName["wallet.proofs.manual_checks_total"].Data.(metricdata.Sum[int64])
	require.True(t, ok)
	result, _ := checks.DataPoints[0].Attributes.Value("result")
	require.Equal(t, metrics.ProofCheckNotFound, result.AsString())

	applied, ok := byName["wallet.proofs.applied_total"].Data.(metricdata.Sum[int64])
	require.True(t, ok)
	source, _ := applied.DataPoints[0].Attributes.Value("source")
	require.Equal(t, metrics.ProofSourceSSE, source.AsString())
}
