package services

import (
	"log/slog"
	"sync"

	"github.com/bsv-blockchain/go-chaintracks/chaintracks"
)

// tipBroadcaster allows multiple subscribers to receive new tip events
type tipBroadcaster struct {
	mu          sync.RWMutex
	subscribers map[chan *chaintracks.BlockHeader]any
	logger      *slog.Logger
}

func newTipBroadcaster(logger *slog.Logger) *tipBroadcaster {
	return &tipBroadcaster{
		logger:      logger,
		subscribers: make(map[chan *chaintracks.BlockHeader]any, 0),
	}
}

// Subscribe registers a user-provided channel to receive new tip events.
// The caller is responsible for creating the channel with an appropriate buffer size
// and closing it after unsubscribing.
// Returns an unsubscribe function that removes the channel from the subscriber list.
func (t *tipBroadcaster) Subscribe(ch chan *chaintracks.BlockHeader) func() {
	t.mu.Lock()
	t.subscribers[ch] = struct{}{}
	t.mu.Unlock()

	return func() {
		t.mu.Lock()
		delete(t.subscribers, ch)
		t.mu.Unlock()
	}
}

// broadcast sends the event to all subscribers.
// If a subscriber's channel is full, the event is dropped for that subscriber.
func (t *tipBroadcaster) broadcast(tip *chaintracks.BlockHeader) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	for sub := range t.subscribers {
		select {
		case sub <- tip:
		default:
			t.logger.Warn("new tip subscriber channel full, dropping event",
				"tip hash", tip.Hash.String(),
				"tip height", tip.Height,
				"tip header", tip.String(),
			)
		}
	}
}
