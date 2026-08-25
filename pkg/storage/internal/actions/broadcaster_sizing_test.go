package actions

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/service"
)

// TestResolveBroadcasterSizing pins the precedence between the operator's explicit
// sizing and the one derived from the throughput strategy. The case that matters
// most is the last one: an untouched config must still produce exactly what every
// deployment gets today.
func TestResolveBroadcasterSizing(t *testing.T) {
	throughputOn := ThroughputConfig{Enabled: true, TargetTPS: 1000}

	tests := map[string]struct {
		explicit        defs.BackgroundBroadcaster
		throughput      ThroughputConfig
		wantWorkers     int
		wantChannelSize int
	}{
		"nothing configured, no throughput: package defaults apply": {
			// Zero Sizing means service.Sizing falls back to its own constants.
			wantWorkers:     0,
			wantChannelSize: 0,
		},
		"throughput only: sizing is derived": {
			throughput:      throughputOn,
			wantWorkers:     256,
			wantChannelSize: 256 * 400,
		},
		"queue widened on a non-throughput deployment": {
			// The incident case: the queue is the lever, and it has to be reachable
			// without switching the whole funding strategy on.
			explicit:        defs.BackgroundBroadcaster{ChannelSize: 5000},
			wantWorkers:     0,
			wantChannelSize: 5000,
		},
		"pool widened, queue left derived": {
			explicit:        defs.BackgroundBroadcaster{Workers: 50},
			throughput:      throughputOn,
			wantWorkers:     50,
			wantChannelSize: 256 * 400,
		},
		"explicit values win over the derivation": {
			explicit:        defs.BackgroundBroadcaster{Workers: 64, ChannelSize: 9000},
			throughput:      throughputOn,
			wantWorkers:     64,
			wantChannelSize: 9000,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			sizing := resolveBroadcasterSizing(tc.explicit, tc.throughput)

			assert.Equal(t, tc.wantWorkers, sizing.Workers)
			assert.Equal(t, tc.wantChannelSize, sizing.ChannelSize)
		})
	}
}

// TestResolveBroadcasterSizingKeepsTodaysDefaults spells out what the zero value
// resolves to once service.Sizing has applied its fallbacks, so a change to either
// side of that contract fails here rather than silently in production.
func TestResolveBroadcasterSizingKeepsTodaysDefaults(t *testing.T) {
	sizing := resolveBroadcasterSizing(defs.BackgroundBroadcaster{}, ThroughputConfig{})

	bb := service.NewBackgroundBroadcaster(t.Context(), nil, nil, nil, sizing)
	depth, capacity := bb.QueueStats()

	assert.Zero(t, depth)
	assert.Equal(t, service.BackgroundBroadcasterChannelSize, capacity)
}
