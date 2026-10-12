package defs

import "fmt"

// CacheType selects the backend of the shared cache store.
type CacheType string

const (
	// CacheMemory keeps cached values in process memory (default).
	CacheMemory CacheType = "memory"
	// CacheRedis keeps cached values in Redis, shared by every instance pointing at it.
	CacheRedis CacheType = "redis"
	// CacheNone disables caching everywhere the cache store is used.
	CacheNone CacheType = "none"
)

// Defaults for the cache store.
const (
	DefaultCacheSize        = 10_000
	DefaultCacheRedisPrefix = "wtb"
)

// Cache configures the cache store shared by components that cache values.
// Each component uses its own namespace; how long entries live is decided by the component.
// An empty Type means memory, and zero Size or Redis.Prefix fall back to the defaults above.
type Cache struct {
	Type CacheType `mapstructure:"type"` // "memory", "redis" or "none"
	// Size bounds the number of entries per namespace (memory only).
	Size  int              `mapstructure:"size"`
	Redis CacheRedisConfig `mapstructure:"redis"`
}

// CacheRedisConfig configures the Redis backend of the cache store.
type CacheRedisConfig struct {
	// URL in go-redis ParseURL form: redis://user:pass@host:6379/0, or rediss:// for TLS.
	URL string `mapstructure:"url"`
	// Prefix is prepended to every key, so several deployments can share one Redis.
	Prefix string `mapstructure:"prefix"`
}

// DefaultCache returns the default cache store configuration.
func DefaultCache() Cache {
	return Cache{
		Type:  CacheMemory,
		Size:  DefaultCacheSize,
		Redis: CacheRedisConfig{Prefix: DefaultCacheRedisPrefix},
	}
}

// Validate checks the cache store configuration.
func (c *Cache) Validate() error {
	if c.Size < 0 {
		return fmt.Errorf("size cannot be negative")
	}

	switch c.Type {
	case "", CacheMemory, CacheNone:
		return nil
	case CacheRedis:
		if c.Redis.URL == "" {
			return fmt.Errorf("redis.url is required when type is 'redis'")
		}
		return nil
	default:
		return fmt.Errorf("invalid cache type: %s (must be 'memory', 'redis' or 'none')", c.Type)
	}
}
