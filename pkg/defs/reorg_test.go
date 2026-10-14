package defs_test

import (
	"testing"
	"time"

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
		"default (immediate)":        {mutate: func(*defs.Reorg) {}},
		"delayed with defaults":      {mutate: func(r *defs.Reorg) { *r = delayed }},
		"mode is case insensitive":   {mutate: func(r *defs.Reorg) { r.Mode = "DELAYED" }},
		"unknown mode":               {mutate: func(r *defs.Reorg) { r.Mode = "later" }, wantErr: true},
		"immediate ignores zero age": {mutate: func(r *defs.Reorg) { r.AgeSeconds = 0 }},
		"delayed requires age":       {mutate: func(r *defs.Reorg) { *r = delayed; r.AgeSeconds = 0 }, wantErr: true},
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

func TestReorgReproveDelay(t *testing.T) {
	// given:
	cfg := defs.DefaultReorg()

	// then: immediate mode never delays, whatever age_seconds says
	require.Zero(t, cfg.ReproveDelay())

	// when:
	cfg.Mode = defs.ReorgModeDelayed

	// then:
	require.Equal(t, 10*time.Minute, cfg.ReproveDelay())
}
