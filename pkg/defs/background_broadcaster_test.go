package defs_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
)

// TestDefaultBackgroundBroadcasterIsUnconfigured pins the contract every existing
// deployment relies on: with nothing in the config file the sizing is still the one
// derived from the strategy, not something this struct imposes.
func TestDefaultBackgroundBroadcasterIsUnconfigured(t *testing.T) {
	cfg := defs.DefaultBackgroundBroadcaster()

	assert.Zero(t, cfg.Workers)
	assert.Zero(t, cfg.ChannelSize)
	require.NoError(t, cfg.Validate())
}

func TestBackgroundBroadcasterValidate(t *testing.T) {
	tests := map[string]struct {
		cfg     defs.BackgroundBroadcaster
		wantErr bool
	}{
		"zero values are the unconfigured default": {
			cfg: defs.BackgroundBroadcaster{},
		},
		"only the queue widened": {
			cfg: defs.BackgroundBroadcaster{ChannelSize: 5000},
		},
		"only the pool widened": {
			cfg: defs.BackgroundBroadcaster{Workers: 50},
		},
		"both at their upper bound": {
			cfg: defs.BackgroundBroadcaster{
				Workers:     defs.MaxBroadcasterWorkers,
				ChannelSize: defs.MaxBroadcasterChannelSize,
			},
		},
		"too many workers": {
			cfg:     defs.BackgroundBroadcaster{Workers: defs.MaxBroadcasterWorkers + 1},
			wantErr: true,
		},
		// A queue is sized in parsed BEEFs, not bytes, so a typo here costs memory
		// rather than a harmless over-allocation.
		"queue beyond the memory guard": {
			cfg:     defs.BackgroundBroadcaster{ChannelSize: defs.MaxBroadcasterChannelSize + 1},
			wantErr: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}
