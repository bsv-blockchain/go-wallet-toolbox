package defs_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
)

func TestReorgValidate(t *testing.T) {
	delayed := defs.DefaultReorg()
	delayed.Mode = defs.ReorgModeDelayed

	tests := map[string]struct {
		mutate  func(*defs.Reorg)
		wantErr bool
	}{
		"default (immediate)":             {mutate: func(*defs.Reorg) {}},
		"delayed with defaults":           {mutate: func(r *defs.Reorg) { *r = delayed }},
		"mode is case insensitive":        {mutate: func(r *defs.Reorg) { r.Mode = "DELAYED" }},
		"unknown mode":                    {mutate: func(r *defs.Reorg) { r.Mode = "later" }, wantErr: true},
		"immediate ignores zero age":      {mutate: func(r *defs.Reorg) { r.AgeSeconds = 0 }},
		"delayed requires age":            {mutate: func(r *defs.Reorg) { *r = delayed; r.AgeSeconds = 0 }, wantErr: true},
		"delayed requires retries":        {mutate: func(r *defs.Reorg) { *r = delayed; r.MaxRetries = 0 }, wantErr: true},
		"delayed requires queue capacity": {mutate: func(r *defs.Reorg) { *r = delayed; r.MaxQueuedHeaders = 0 }, wantErr: true},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// given:
			cfg := defs.DefaultReorg()
			test.mutate(&cfg)

			// when:
			err := cfg.Validate()

			// then:
			if test.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
