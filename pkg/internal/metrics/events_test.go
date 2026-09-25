package metrics_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/metrics"
)

func TestEventQueueMetrics(t *testing.T) {
	// given: two reorg subscriber queues and one proven queue
	unregister, err := metrics.RegisterEventQueueGauges(func() []metrics.EventQueueStat {
		return []metrics.EventQueueStat{
			{Stream: "reorg", Depth: 3, OldestAge: time.Second},
			{Stream: "reorg", Depth: 4, OldestAge: 5 * time.Second},
			{Stream: "tx_proven", Depth: 10, OldestAge: 2 * time.Second},
		}
	})
	require.NoError(t, err)
	defer unregister()

	// and: some undelivered events
	metrics.RecordEventsUndelivered(t.Context(), "tx_proven", metrics.EventUndeliveredShutdownTimeout, 2)
	metrics.RecordEventsUndelivered(t.Context(), "tx_proven", metrics.EventUndeliveredShutdownTimeout, 0)

	// when:
	byName := collect(t)

	// then: depth is summed per stream
	depth, ok := byName["wallet.events.queue_depth"].Data.(metricdata.Gauge[int64])
	require.True(t, ok)
	depthByStream := map[string]int64{}
	for _, dp := range depth.DataPoints {
		stream, _ := dp.Attributes.Value(attribute.Key("stream"))
		depthByStream[stream.AsString()] = dp.Value
	}
	assert.Equal(t, map[string]int64{"reorg": 7, "tx_proven": 10}, depthByStream)

	// and: oldest age is the worst per stream
	oldest, ok := byName["wallet.events.queue_oldest_age_seconds"].Data.(metricdata.Gauge[float64])
	require.True(t, ok)
	oldestByStream := map[string]float64{}
	for _, dp := range oldest.DataPoints {
		stream, _ := dp.Attributes.Value(attribute.Key("stream"))
		oldestByStream[stream.AsString()] = dp.Value
	}
	assert.Equal(t, map[string]float64{"reorg": 5, "tx_proven": 2}, oldestByStream)

	// and: undelivered counts only non-zero records
	undelivered, ok := byName["wallet.events.undelivered"].Data.(metricdata.Sum[int64])
	require.True(t, ok)
	require.Len(t, undelivered.DataPoints, 1)
	assert.EqualValues(t, 2, undelivered.DataPoints[0].Value)
	reason, _ := undelivered.DataPoints[0].Attributes.Value(attribute.Key("reason"))
	assert.Equal(t, metrics.EventUndeliveredShutdownTimeout, reason.AsString())
}
