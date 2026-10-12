package cachestore_test

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/cachestore"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
)

// TestBackends runs the same contract against every backend.
func TestBackends(t *testing.T) {
	backends := map[string]func(t *testing.T) cachestore.Provider{
		"memory": func(*testing.T) cachestore.Provider { return cachestore.NewMemory(10) },
		"redis": func(t *testing.T) cachestore.Provider {
			return newRedis(t, slog.New(slog.DiscardHandler), miniredis.RunT(t))
		},
	}

	for name, newProvider := range backends {
		t.Run(name, func(t *testing.T) {
			t.Run("get returns what was set", func(t *testing.T) {
				store := newProvider(t).Store("ns")

				require.NoError(t, store.Set(t.Context(), "k", []byte("v"), time.Minute))
				v, ok, err := store.Get(t.Context(), "k")

				require.NoError(t, err)
				require.True(t, ok)
				require.Equal(t, "v", string(v))
			})

			t.Run("missing key is a miss, not an error", func(t *testing.T) {
				_, ok, err := newProvider(t).Store("ns").Get(t.Context(), "nope")

				require.NoError(t, err)
				require.False(t, ok)
			})

			t.Run("namespaces are isolated, including purge", func(t *testing.T) {
				p := newProvider(t)
				a, b := p.Store("a"), p.Store("b")
				require.NoError(t, a.Set(t.Context(), "k", []byte("a"), time.Minute))
				require.NoError(t, b.Set(t.Context(), "k", []byte("b"), time.Minute))

				require.NoError(t, a.Purge(t.Context()))

				_, okA, _ := a.Get(t.Context(), "k")
				vB, okB, _ := b.Get(t.Context(), "k")
				require.False(t, okA)
				require.True(t, okB)
				require.Equal(t, "b", string(vB))
			})

			t.Run("same namespace is shared", func(t *testing.T) {
				p := newProvider(t)
				require.NoError(t, p.Store("ns").Set(t.Context(), "k", nil, time.Minute))

				_, ok, err := p.Store("ns").Get(t.Context(), "k")

				require.NoError(t, err)
				require.True(t, ok)
			})
		})
	}
}

func TestMemory_ExpiresAndEvicts(t *testing.T) {
	store := cachestore.NewMemory(2).Store("ns")

	require.NoError(t, store.Set(t.Context(), "short", nil, time.Millisecond))
	time.Sleep(5 * time.Millisecond)
	_, ok, _ := store.Get(t.Context(), "short")
	require.False(t, ok, "expired entry must miss")

	for _, k := range []string{"a", "b", "c"} {
		require.NoError(t, store.Set(t.Context(), k, nil, 0))
	}
	_, ok, _ = store.Get(t.Context(), "a")
	require.False(t, ok, "oldest entry must be evicted beyond size")
}

func TestRedis_Expires(t *testing.T) {
	srv := miniredis.RunT(t)
	store := newRedis(t, slog.New(slog.DiscardHandler), srv).Store("ns")
	require.NoError(t, store.Set(t.Context(), "k", nil, time.Minute))

	srv.FastForward(time.Minute)

	_, ok, err := store.Get(t.Context(), "k")
	require.NoError(t, err)
	require.False(t, ok)
}

func TestRedis_SharedBetweenProviders(t *testing.T) {
	srv := miniredis.RunT(t)
	first, second := newRedis(t, slog.New(slog.DiscardHandler), srv), newRedis(t, slog.New(slog.DiscardHandler), srv)
	require.NoError(t, first.Store("ns").Set(t.Context(), "k", []byte("v"), time.Minute))

	v, ok, err := second.Store("ns").Get(t.Context(), "k")

	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "v", string(v))
}

func TestRedis_OutageIsAnErrorAndWarnsOncePerInterval(t *testing.T) {
	srv := miniredis.RunT(t)
	var logs bytes.Buffer
	store := newRedis(t, slog.New(slog.NewTextHandler(&logs, nil)), srv).Store("ns")
	srv.Close()

	for range 3 {
		_, ok, err := store.Get(t.Context(), "k")
		require.Error(t, err)
		require.False(t, ok)
		require.Error(t, store.Set(t.Context(), "k", nil, time.Minute))
	}

	require.Equal(t, 1, strings.Count(logs.String(), "level=WARN"))
}

func TestNew(t *testing.T) {
	logger := logging.NewTestLogger(t)

	none, err := cachestore.New(logger, defs.Cache{Type: defs.CacheNone})
	require.NoError(t, err)
	store := none.Store("ns")
	require.NoError(t, store.Set(t.Context(), "k", nil, time.Minute))
	_, ok, err := store.Get(t.Context(), "k")
	require.NoError(t, err)
	require.False(t, ok, "none must never hit")

	_, err = cachestore.New(logger, defs.Cache{})
	require.NoError(t, err, "empty type means memory")

	_, err = cachestore.New(logger, defs.Cache{Type: "memcached"})
	require.Error(t, err)

	_, err = cachestore.New(logger, defs.Cache{Type: defs.CacheRedis, Redis: defs.CacheRedisConfig{URL: "not a url"}})
	require.Error(t, err)
}

func newRedis(t *testing.T, logger *slog.Logger, srv *miniredis.Miniredis) cachestore.Provider {
	p, err := cachestore.New(logger, defs.Cache{Type: defs.CacheRedis, Redis: defs.CacheRedisConfig{URL: "redis://" + srv.Addr()}})
	require.NoError(t, err)
	return p
}
