package cachestore

import (
	"context"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
)

// NewMemory returns an in-process provider; each namespace is its own LRU of size entries.
func NewMemory(size int) Provider {
	if size <= 0 {
		size = defs.DefaultCacheSize
	}
	return &memoryProvider{size: size, stores: make(map[string]*memoryStore)}
}

type memoryProvider struct {
	size   int
	mu     sync.Mutex
	stores map[string]*memoryStore
}

// Store returns the same store for the same namespace, so two components asking for one
// namespace share entries just as they would on Redis.
func (p *memoryProvider) Store(namespace string) Store {
	p.mu.Lock()
	defer p.mu.Unlock()

	if s, ok := p.stores[namespace]; ok {
		return s
	}
	l, _ := lru.New[string, memoryEntry](p.size) // only fails for size <= 0, excluded above
	s := &memoryStore{lru: l}
	p.stores[namespace] = s
	return s
}

type memoryEntry struct {
	value     []byte
	expiresAt time.Time // zero = no expiry
}

type memoryStore struct {
	lru *lru.Cache[string, memoryEntry]
}

func (s *memoryStore) Get(_ context.Context, key string) ([]byte, bool, error) {
	e, ok := s.lru.Get(key)
	if !ok {
		return nil, false, nil
	}
	if !e.expiresAt.IsZero() && time.Now().After(e.expiresAt) {
		s.lru.Remove(key)
		return nil, false, nil
	}
	return e.value, true, nil
}

func (s *memoryStore) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	e := memoryEntry{value: value}
	if ttl > 0 {
		e.expiresAt = time.Now().Add(ttl)
	}
	s.lru.Add(key, e)
	return nil
}

func (s *memoryStore) Purge(context.Context) error {
	s.lru.Purge()
	return nil
}
