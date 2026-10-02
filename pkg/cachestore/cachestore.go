// Package cachestore is a small key-value cache shared by components that want to
// avoid repeating expensive lookups. The backend (memory or Redis) is chosen once in
// config; each component takes its own namespace from the Provider.
package cachestore

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
)

// Store is one namespace of the cache. Keys are local to the namespace.
//
// Errors mean the backend could not be reached; callers should treat them as a miss
// (or a skipped write) rather than fail, since the cache is never the source of truth.
type Store interface {
	// Get returns the value and true, or false when the key is absent or expired.
	Get(ctx context.Context, key string) ([]byte, bool, error)
	// Set stores the value; a ttl of 0 or less keeps it until evicted or purged.
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	// Purge drops every entry of this namespace, leaving other namespaces untouched.
	Purge(ctx context.Context) error
}

// Provider hands out namespaced stores that share one backend.
type Provider interface {
	Store(namespace string) Store
}

// New builds the provider for the configured backend.
func New(logger *slog.Logger, cfg defs.Cache) (Provider, error) {
	switch cfg.Type {
	case defs.CacheNone:
		return noopProvider{}, nil
	case defs.CacheRedis:
		return newRedisProvider(logger, cfg.Redis)
	case "", defs.CacheMemory:
		return NewMemory(cfg.Size), nil
	default:
		return nil, fmt.Errorf("unknown cache type %q", cfg.Type)
	}
}

type noopProvider struct{}

func (noopProvider) Store(string) Store { return noopStore{} }

type noopStore struct{}

func (noopStore) Get(context.Context, string) ([]byte, bool, error)        { return nil, false, nil }
func (noopStore) Set(context.Context, string, []byte, time.Duration) error { return nil }
func (noopStore) Purge(context.Context) error                              { return nil }
