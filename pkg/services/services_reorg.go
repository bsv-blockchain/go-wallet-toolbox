package services

import (
	"log/slog"
	"sync"

	"github.com/bsv-blockchain/go-chaintracks/chaintracks"
)

// reorgBroadcaster allows multiple subscribers to receive reorg events
type reorgBroadcaster struct {
	mu          sync.RWMutex
	subscribers map[chan *chaintracks.ReorgEvent]any
	logger      *slog.Logger
}

func newReorgBroadcaster(logger *slog.Logger) *reorgBroadcaster {
	return &reorgBroadcaster{
		logger:      logger,
		subscribers: make(map[chan *chaintracks.ReorgEvent]any, 0),
	}
}

// Subscribe registers a user-provided channel to receive reorg events.
// The caller is responsible for creating the channel with an appropriate buffer size
// and closing it after unsubscribing.
// Returns an unsubscribe function that removes the channel from the subscriber list.
func (b *reorgBroadcaster) Subscribe(ch chan *chaintracks.ReorgEvent) func() {
	b.mu.Lock()
	b.subscribers[ch] = struct{}{}
	b.mu.Unlock()

	return func() {
		b.mu.Lock()
		delete(b.subscribers, ch)
		b.mu.Unlock()
	}
}

// broadcast sends the event to all subscribers.
// If a subscriber's channel is full, the event is dropped for that subscriber.
func (b *reorgBroadcaster) broadcast(event *chaintracks.ReorgEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for sub := range b.subscribers {
		select {
		case sub <- event:
		default:
			b.logger.Warn("reorg subscriber channel full, dropping event",
				"depth", event.Depth,
				"orphaned hashes", event.OrphanedHashes)
		}
	}
}
