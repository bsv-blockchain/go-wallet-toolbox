package pending

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/to"
)

// SignActionLocalRepository is a cache for storing pending sign actions with a configurable time-to-live (TTL).
type SignActionLocalRepository struct {
	actions sync.Map
	logger  *slog.Logger

	nextCleanup *time.Time
	ttl         time.Duration
}

// NewSignActionLocalRepository initializes a new SignActionLocalRepository with the given logger and TTL.
func NewSignActionLocalRepository(logger *slog.Logger, ttl time.Duration) *SignActionLocalRepository {
	logger = logging.Child(logger, "SignActionLocalRepository")

	return &SignActionLocalRepository{
		actions: sync.Map{},
		logger:  logger,
		ttl:     ttl,
	}
}

type pendingSignActionItem struct {
	action    SignAction
	timestamp time.Time
}

// Save stores a pending sign action in the cache using the provided reference as a key. If TTL is set, it checks for cleanup.
func (l *SignActionLocalRepository) Save(reference string, action *SignAction) error {
	if l.ttl > 0 {
		l.checkForCleanup()
	}

	l.actions.Store(reference, pendingSignActionItem{
		action:    *action,
		timestamp: time.Now(),
	})
	return nil
}

// Get retrieves a pending sign action from the cache using the provided reference as a key.
// Returns the pending sign action if found, or an error if not found.
func (l *SignActionLocalRepository) Get(reference string) (*SignAction, error) {
	item, ok := l.actions.Load(reference)
	if !ok {
		return nil, fmt.Errorf("no action found for reference %s: %w", reference, wdk.ErrNotFoundError)
	}

	action := item.(pendingSignActionItem).action

	return &action, nil
}

// Delete removes the pending sign action from the cache that corresponds to the given reference. Returns an error if any occurs.
func (l *SignActionLocalRepository) Delete(reference string) error {
	l.actions.Delete(reference)
	return nil
}

func (l *SignActionLocalRepository) checkForCleanup() {
	if l.nextCleanup == nil {
		l.nextCleanup = to.Ptr(time.Now().Add(l.ttl).Add(time.Second))
		return
	}

	if time.Now().After(*l.nextCleanup) {
		l.cleanup()
		l.nextCleanup = to.Ptr(time.Now().Add(l.ttl).Add(time.Second))
	}
}

func (l *SignActionLocalRepository) cleanup() {
	l.logger.Info("cleaning up old pending sign actions cache")

	cutoff := time.Now().Add(-l.ttl)
	l.actions.Range(func(key, value any) bool {
		item := value.(pendingSignActionItem)
		if item.timestamp.Before(cutoff) {
			l.logger.Info("removing expired pending sign action", slog.String("reference", key.(string)))
			l.actions.Delete(key)
		}
		return true
	})
}
