package defs_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
)

func TestEffectiveProofCheckAfterBlocks(t *testing.T) {
	tests := map[string]struct {
		configured uint
		sseProofs  bool
		expected   uint
	}{
		"default with SSE waits two blocks":          {configured: 2, sseProofs: true, expected: 2},
		"configured value is used with SSE":          {configured: 5, sseProofs: true, expected: 5},
		"zero with SSE polls on every run":           {configured: 0, sseProofs: true, expected: 0},
		"without SSE every run polls, as before":     {configured: 2, sseProofs: false, expected: 0},
		"without SSE a configured value is not used": {configured: 5, sseProofs: false, expected: 0},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			cfg := defs.DefaultSynchronizeTxStatuses()
			cfg.ProofCheckAfterBlocks = test.configured

			require.Equal(t, test.expected, cfg.EffectiveProofCheckAfterBlocks(test.sseProofs))
		})
	}
}
