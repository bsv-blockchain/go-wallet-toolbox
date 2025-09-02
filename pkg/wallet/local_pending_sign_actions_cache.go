package wallet

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/to"
)

type LocalPendingSignActionsCache struct {
	actions sync.Map
	logger  *slog.Logger

	nextCleanup *time.Time
	ttl         time.Duration
}

func NewLocalPendingSignActionsCache(logger *slog.Logger, ttl time.Duration) *LocalPendingSignActionsCache {
	logger = logging.Child(logger, "LocalPendingSignActionsCache")

	return &LocalPendingSignActionsCache{
		actions: sync.Map{},
		logger:  logger,
		ttl:     ttl,
	}
}

type pendingSignActionItem struct {
	action    wdk.PendingSignAction
	timestamp time.Time
}

func (l *LocalPendingSignActionsCache) Set(reference string, action *wdk.PendingSignAction) error {
	if l.ttl > 0 {
		l.checkForCleanup()
	}

	l.actions.Store(reference, pendingSignActionItem{
		action:    *action,
		timestamp: time.Now(),
	})
	return nil
}

func (l *LocalPendingSignActionsCache) Get(reference string) (*wdk.PendingSignAction, error) {
	item, ok := l.actions.Load(reference)
	if !ok {
		return nil, fmt.Errorf("no action found for reference %s: %w", reference, wdk.NotFoundError)
	}

	action := item.(pendingSignActionItem).action

	return &action, nil
}

func (l *LocalPendingSignActionsCache) Delete(reference string) error {
	l.actions.Delete(reference)
	return nil
}

func (l *LocalPendingSignActionsCache) checkForCleanup() {
	if l.nextCleanup == nil {
		l.nextCleanup = to.Ptr(time.Now().Add(l.ttl).Add(time.Second))
		return
	}

	if time.Now().After(*l.nextCleanup) {
		l.cleanup()
		l.nextCleanup = to.Ptr(time.Now().Add(l.ttl).Add(time.Second))
	}
}

func (l *LocalPendingSignActionsCache) cleanup() {
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
