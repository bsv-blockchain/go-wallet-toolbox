package defs_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
)

func TestCache_Validate(t *testing.T) {
	cases := map[string]struct {
		cfg     defs.Cache
		wantErr bool
	}{
		"default":            {cfg: defs.DefaultCache()},
		"empty means memory": {cfg: defs.Cache{}},
		"none":               {cfg: defs.Cache{Type: defs.CacheNone}},
		"redis with url":     {cfg: defs.Cache{Type: defs.CacheRedis, Redis: defs.CacheRedisConfig{URL: "redis://localhost:6379"}}},
		"redis without url":  {cfg: defs.Cache{Type: defs.CacheRedis}, wantErr: true},
		"unknown type":       {cfg: defs.Cache{Type: "memcached"}, wantErr: true},
		"negative size":      {cfg: defs.Cache{Size: -1}, wantErr: true},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
