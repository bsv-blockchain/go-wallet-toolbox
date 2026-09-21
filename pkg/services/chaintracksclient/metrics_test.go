package chaintracksclient_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/bsv-blockchain/go-chaintracks/chaintracks"
	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracksclient"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracksclient/testabilities"
)

// metricsReader is installed once for the whole package: the chain instruments
// are created lazily against the global delegating provider, which forwards
// only to the first SDK provider set. Other tests in this package emit tips and
// reorgs too, so assertions below compare before/after deltas.
var metricsReader = sdkmetric.NewManualReader()

func TestMain(m *testing.M) {
	otel.SetMeterProvider(sdkmetric.NewMeterProvider(sdkmetric.WithReader(metricsReader)))
	os.Exit(m.Run())
}

type chainMetricsSnapshot struct {
	tipHeight     int64
	tipSeen       bool
	reorgs        int64
	depthCount    uint64
	depthSum      int64
	depthBuckets  []uint64
	depthBoundary []float64
}

func collectChainMetrics(t *testing.T) chainMetricsSnapshot {
	t.Helper()
	var data metricdata.ResourceMetrics
	require.NoError(t, metricsReader.Collect(t.Context(), &data))

	var snap chainMetricsSnapshot
	for _, scope := range data.ScopeMetrics {
		for _, m := range scope.Metrics {
			switch m.Name {
			case "wallet.chain.tip_height":
				gauge, ok := m.Data.(metricdata.Gauge[int64])
				require.True(t, ok)
				if len(gauge.DataPoints) > 0 {
					snap.tipHeight = gauge.DataPoints[0].Value
					snap.tipSeen = true
				}
			case "wallet.chain.reorgs_total":
				sum, ok := m.Data.(metricdata.Sum[int64])
				require.True(t, ok)
				for _, dp := range sum.DataPoints {
					snap.reorgs += dp.Value
				}
			case "wallet.chain.reorg_depth_blocks":
				hist, ok := m.Data.(metricdata.Histogram[int64])
				require.True(t, ok)
				for _, dp := range hist.DataPoints {
					snap.depthCount += dp.Count
					snap.depthSum += dp.Sum
					snap.depthBuckets = dp.BucketCounts
					snap.depthBoundary = dp.Bounds
				}
			}
		}
	}
	return snap
}

func bucketDelta(before, after []uint64, idx int) uint64 {
	var prev uint64
	if idx < len(before) {
		prev = before[idx]
	}
	return after[idx] - prev
}

func TestService_RecordsChainMetrics(t *testing.T) {
	// given:
	mockCT := testabilities.NewMockChaintracks()

	service, err := chaintracksclient.New(
		logging.NewTestLogger(t),
		nil,
		chaintracksclient.WithChaintracks(mockCT),
	)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	tips := make(chan *chaintracks.BlockHeader, 4)
	reorgs := make(chan *chaintracks.ReorgEvent, 4)

	// and: callbacks that fail, to prove metrics do not depend on callback success
	err = service.Start(ctx, chaintracksclient.Callbacks{
		OnTip: func(header *chaintracks.BlockHeader) error {
			tips <- header
			return errors.New("tip callback failure")
		},
		OnReorg: func(event *chaintracks.ReorgEvent) error {
			reorgs <- event
			return errors.New("reorg callback failure")
		},
	})
	require.NoError(t, err)

	before := collectChainMetrics(t)

	// when: two tips arrive
	for _, height := range []uint32{900_000, 900_001} {
		mockCT.SendTip(&chaintracks.BlockHeader{Height: height})
		select {
		case <-tips:
		case <-time.After(time.Second):
			t.Fatalf("tip %d not delivered", height)
		}
	}

	// and: a 1-block reorg and a 3-block reorg arrive
	for _, depth := range []uint32{1, 3} {
		mockCT.SendReorg(&chaintracks.ReorgEvent{
			Depth:  depth,
			NewTip: &chaintracks.BlockHeader{Height: 900_001},
		})
		select {
		case <-reorgs:
		case <-time.After(time.Second):
			t.Fatalf("reorg of depth %d not delivered", depth)
		}
	}

	// and: a reorg without a new tip arrives (skipped before the callback)
	mockCT.SendReorg(&chaintracks.ReorgEvent{
		Depth:          2,
		OrphanedHashes: []chainhash.Hash{{0x01}, {0x02}},
	})

	// then: every reorg, including the incomplete one, is counted
	var after chainMetricsSnapshot
	require.Eventually(t, func() bool {
		after = collectChainMetrics(t)
		return after.reorgs-before.reorgs == 3
	}, time.Second, 10*time.Millisecond, "should count 3 reorgs")

	// and: tip gauge holds the latest height
	require.True(t, after.tipSeen)
	assert.EqualValues(t, 900_001, after.tipHeight)

	// and: depth histogram captures each reorg's depth
	assert.EqualValues(t, 3, after.depthCount-before.depthCount)
	assert.EqualValues(t, 6, after.depthSum-before.depthSum, "1 + 3 + 2")
	require.Equal(t, []float64{1, 2, 3, 5, 10, 20, 50, 100}, after.depthBoundary)
	assert.EqualValues(t, 1, bucketDelta(before.depthBuckets, after.depthBuckets, 0), "depth 1 in (-inf,1]")
	assert.EqualValues(t, 1, bucketDelta(before.depthBuckets, after.depthBuckets, 1), "depth 2 in (1,2]")
	assert.EqualValues(t, 1, bucketDelta(before.depthBuckets, after.depthBuckets, 2), "depth 3 in (2,3]")
}
