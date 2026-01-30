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

// Subscribe returns a channel that receives reorg events and an unsubscribe function.
// The channel is buffered to avoid blocking the broadcaster.
// Call the returned unsubscribe function to stop receiving events and close the channel.
func (b *reorgBroadcaster) Subscribe() (<-chan *chaintracks.ReorgEvent, func()) {
	ch := make(chan *chaintracks.ReorgEvent, 10)

	b.mu.Lock()
	b.subscribers[ch] = struct{}{}
	b.mu.Unlock()

	unsubscribe := func() {
		b.mu.Lock()
		delete(b.subscribers, ch)
		close(ch)
		b.mu.Unlock()
	}

	return ch, unsubscribe
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
