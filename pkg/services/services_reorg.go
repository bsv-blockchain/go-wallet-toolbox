package services

import (
	"log/slog"
	"sync"

	"github.com/bsv-blockchain/go-chaintracks/chaintracks"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/eventqueue"
)

// reorgBroadcaster allows multiple subscribers to receive reorg events
type reorgBroadcaster struct {
	mu          sync.RWMutex
	subscribers map[chan *chaintracks.ReorgEvent]*eventqueue.Queue[*chaintracks.ReorgEvent]
	logger      *slog.Logger
}

func newReorgBroadcaster(logger *slog.Logger) *reorgBroadcaster {
	return &reorgBroadcaster{
		logger:      logger,
		subscribers: make(map[chan *chaintracks.ReorgEvent]*eventqueue.Queue[*chaintracks.ReorgEvent]),
	}
}

// Subscribe registers a user-provided channel to receive reorg events.
// Events are never dropped and the chaintracks event loop never waits for the
// subscriber: events the channel cannot take yet are buffered in memory and
// delivered in order. The caller owns the channel and may close it once the
// unsubscribe function has returned.
// Returns an unsubscribe function that removes the channel from the subscriber
// list and discards events not yet delivered to it.
func (b *reorgBroadcaster) Subscribe(ch chan *chaintracks.ReorgEvent) func() {
	b.mu.Lock()
	queue, ok := b.subscribers[ch]
	if !ok {
		queue = eventqueue.New(eventqueue.StreamReorg, chan<- *chaintracks.ReorgEvent(ch), b.logger)
		b.subscribers[ch] = queue
	}
	b.mu.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			b.mu.Lock()
			if b.subscribers[ch] == queue {
				delete(b.subscribers, ch)
			}
			b.mu.Unlock()

			// Returns once the queue stopped sending, so the caller may close ch.
			queue.Discard()
		})
	}
}

// broadcast hands the event to every subscriber's queue. It never blocks.
func (b *reorgBroadcaster) broadcast(event *chaintracks.ReorgEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, queue := range b.subscribers {
		queue.Publish(event)
	}
}
