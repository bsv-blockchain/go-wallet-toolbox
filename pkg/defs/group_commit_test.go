package defs_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
)

func TestGroupCommit_DefaultIsOff(t *testing.T) {
	cfg := defs.DefaultDBConfig()

	assert.False(t, cfg.GroupCommit.Enabled)
	assert.Equal(t, defs.DefaultGroupCommitMaxBatch, cfg.GroupCommit.MaxBatch)
	assert.Equal(t, defs.DefaultGroupCommitWorkers, cfg.GroupCommit.Workers)
	assert.Zero(t, cfg.GroupCommit.MaxWait)
	require.NoError(t, cfg.Validate())
}

func TestGroupCommit_ValidateRejectsNegativeValues(t *testing.T) {
	tests := map[string]defs.GroupCommit{
		"max batch": {MaxBatch: -1},
		"max wait":  {MaxWait: -time.Millisecond},
		"workers":   {Workers: -1},
	}
	for name, gc := range tests {
		t.Run(name, func(t *testing.T) {
			cfg := defs.DefaultDBConfig()
			cfg.GroupCommit = gc
			require.Error(t, cfg.Validate())
		})
	}
}
