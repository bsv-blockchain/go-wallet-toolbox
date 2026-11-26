package storage

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
)

func TestWarnIfZeroFeeOnMainnet(t *testing.T) {
	tests := map[string]struct {
		chain    defs.BSVNetwork
		fee      int64
		wantWarn bool
	}{
		"zero fee on mainnet warns":         {chain: defs.NetworkMainnet, fee: 0, wantWarn: true},
		"non-zero fee on mainnet is silent": {chain: defs.NetworkMainnet, fee: 1, wantWarn: false},
		"zero fee on testnet is silent":     {chain: defs.NetworkTestnet, fee: 0, wantWarn: false},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			log := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

			warnIfZeroFeeOnMainnet(log, tt.chain, defs.FeeModel{Type: defs.SatPerKB, Value: tt.fee})

			if tt.wantWarn {
				assert.Contains(t, buf.String(), "level=WARN")
				assert.Contains(t, buf.String(), "0 sat/kb on mainnet")
			} else {
				assert.Empty(t, buf.String())
			}
		})
	}
}
