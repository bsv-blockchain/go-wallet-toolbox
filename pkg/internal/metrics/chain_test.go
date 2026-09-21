package metrics_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/metrics"
)

func TestChainTipAndReorgMetrics(t *testing.T) {
	// when: two tips and two reorgs recorded
	metrics.RecordChainTipHeight(t.Context(), 900_000)
	metrics.RecordChainTipHeight(t.Context(), 900_001)
	metrics.RecordChainReorg(t.Context(), 1)
	metrics.RecordChainReorg(t.Context(), 3)

	byName := collect(t)

	// then: tip gauge holds the latest height
	tip, ok := byName["wallet.chain.tip_height"].Data.(metricdata.Gauge[int64])
	require.True(t, ok)
	require.Len(t, tip.DataPoints, 1)
	assert.EqualValues(t, 900_001, tip.DataPoints[0].Value)

	// and: reorg counter counts every reorg
	reorgs, ok := byName["wallet.chain.reorgs_total"].Data.(metricdata.Sum[int64])
	require.True(t, ok)
	require.Len(t, reorgs.DataPoints, 1)
	assert.EqualValues(t, 2, reorgs.DataPoints[0].Value)

	// and: depth histogram captures the distribution
	depth, ok := byName["wallet.chain.reorg_depth_blocks"].Data.(metricdata.Histogram[int64])
	require.True(t, ok)
	require.Len(t, depth.DataPoints, 1)
	dp := depth.DataPoints[0]
	assert.EqualValues(t, 2, dp.Count)
	assert.EqualValues(t, 4, dp.Sum)
	assert.Equal(t, []float64{1, 2, 3, 5, 10, 20, 50, 100}, dp.Bounds)
	// buckets: (-inf,1] (1,2] (2,3] ...
	assert.EqualValues(t, 1, dp.BucketCounts[0], "depth 1")
	assert.EqualValues(t, 1, dp.BucketCounts[2], "depth 3")
}
