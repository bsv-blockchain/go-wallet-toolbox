package cachestore

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
)

// redisOpTimeout keeps a slow Redis from stalling callers; on timeout the call fails
// and the caller treats it as a miss.
const redisOpTimeout = 200 * time.Millisecond

// redisWarnInterval limits how often an outage is logged at Warn level, so a sustained
// outage shows up in logs without one line per lookup.
const redisWarnInterval = time.Minute

// Keys embed a per-namespace generation number: <prefix>:<namespace>:<gen>:<key>.
// Purge bumps the generation instead of deleting keys, so it is one INCR no matter how
// many keys or instances there are; the old generation's keys expire via their TTL.
// The scripts read the generation and touch the key in one round trip.
var (
	redisGet = redis.NewScript(`
local gen = redis.call('GET', KEYS[1]) or '0'
return redis.call('GET', ARGV[1] .. ':' .. gen .. ':' .. ARGV[2])`)
	redisSet = redis.NewScript(`
local gen = redis.call('GET', KEYS[1]) or '0'
local key = ARGV[1] .. ':' .. gen .. ':' .. ARGV[2]
if tonumber(ARGV[4]) > 0 then
  return redis.call('SET', key, ARGV[3], 'PX', ARGV[4])
end
return redis.call('SET', key, ARGV[3])`)
)

type redisProvider struct {
	logger   *slog.Logger
	client   *redis.Client
	prefix   string
	lastWarn atomic.Int64 // unix nanos of the last Warn-level failure log
}

// ponytail: the client lives for the process; nothing in the toolbox has a shutdown hook to close it.
func newRedisProvider(logger *slog.Logger, cfg defs.CacheRedisConfig) (*redisProvider, error) {
	opts, err := redis.ParseURL(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid cache redis url: %w", err)
	}

	prefix := cfg.Prefix
	if prefix == "" {
		prefix = defs.DefaultCacheRedisPrefix
	}

	p := &redisProvider{logger: logger, client: redis.NewClient(opts), prefix: prefix}

	// An unreachable Redis is not fatal (callers just miss), but say so at startup.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := p.client.Ping(ctx).Err(); err != nil {
		p.lastWarn.Store(time.Now().UnixNano())
		logger.Warn("cache redis is unreachable, nothing will be cached until it is",
			slog.String("addr", opts.Addr), slog.String("error", err.Error()))
	}

	return p, nil
}

func (p *redisProvider) Store(namespace string) Store {
	base := p.prefix + ":" + namespace
	return &redisStore{p: p, base: base, genKey: base + ":gen"}
}

// logFailure logs at Warn at most once per redisWarnInterval and at Debug otherwise.
func (p *redisProvider) logFailure(ctx context.Context, msg string, err error) {
	now := time.Now().UnixNano()
	last := p.lastWarn.Load()
	level := slog.LevelDebug
	if now-last >= int64(redisWarnInterval) && p.lastWarn.CompareAndSwap(last, now) {
		level = slog.LevelWarn
	}
	p.logger.Log(ctx, level, msg, slog.String("error", err.Error()))
}

type redisStore struct {
	p      *redisProvider
	base   string
	genKey string
}

func (s *redisStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, redisOpTimeout)
	defer cancel()

	v, err := redisGet.Run(ctx, s.p.client, []string{s.genKey}, s.base, key).Text()
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		s.p.logFailure(ctx, "cache redis lookup failed", err)
		return nil, false, fmt.Errorf("cache get %s: %w", s.base, err)
	}
	return []byte(v), true, nil
}

func (s *redisStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, redisOpTimeout)
	defer cancel()

	ms := int64(0)
	if ttl > 0 {
		ms = max(ttl.Milliseconds(), 1)
	}
	if err := redisSet.Run(ctx, s.p.client, []string{s.genKey}, s.base, key, value, strconv.FormatInt(ms, 10)).Err(); err != nil {
		s.p.logFailure(ctx, "cache redis store failed", err)
		return fmt.Errorf("cache set %s: %w", s.base, err)
	}
	return nil
}

func (s *redisStore) Purge(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, redisOpTimeout)
	defer cancel()

	if err := s.p.client.Incr(ctx, s.genKey).Err(); err != nil {
		// Always Warn: a missed purge leaves stale entries until their TTL expires.
		s.p.logger.WarnContext(ctx, "cache redis purge failed", slog.String("namespace", s.base), slog.String("error", err.Error()))
		return fmt.Errorf("cache purge %s: %w", s.base, err)
	}
	return nil
}
