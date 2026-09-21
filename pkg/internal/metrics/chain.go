package metrics

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

const chainMeterName = "github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracksclient"

var (
	chainInitOnce sync.Once

	chainTipHeight  metric.Int64Gauge
	chainReorgs     metric.Int64Counter
	chainReorgDepth metric.Int64Histogram
)

func ensureChainInstruments() {
	chainInitOnce.Do(func() {
		meter := otel.Meter(chainMeterName)
		// Instrument creation only fails on invalid names; fall back to no-op
		// instruments rather than propagating an error into the event loop.
		chainTipHeight, _ = meter.Int64Gauge("wallet.chain.tip_height",
			metric.WithUnit("{block}"),
			metric.WithDescription("Height of the latest chain tip broadcast by chaintracks"))
		chainReorgs, _ = meter.Int64Counter("wallet.chain.reorgs_total",
			metric.WithDescription("Chain reorganizations reported by chaintracks"))
		chainReorgDepth, _ = meter.Int64Histogram("wallet.chain.reorg_depth_blocks",
			metric.WithUnit("{block}"),
			metric.WithDescription("Blocks orphaned per chain reorganization"),
			metric.WithExplicitBucketBoundaries(1, 2, 3, 5, 10, 20, 50, 100))
	})
}

// RecordChainTipHeight sets the chain tip height gauge. A flat value over time
// means chaintracks stopped advancing (e.g. lost its P2P or remote source).
func RecordChainTipHeight(ctx context.Context, height uint32) {
	ensureChainInstruments()
	if chainTipHeight != nil {
		chainTipHeight.Record(ctx, int64(height))
	}
}

// RecordChainReorg counts one chain reorganization and records its depth
// (the number of orphaned blocks).
func RecordChainReorg(ctx context.Context, depth uint32) {
	ensureChainInstruments()
	if chainReorgs != nil {
		chainReorgs.Add(ctx, 1)
	}
	if chainReorgDepth != nil {
		chainReorgDepth.Record(ctx, int64(depth))
	}
}
