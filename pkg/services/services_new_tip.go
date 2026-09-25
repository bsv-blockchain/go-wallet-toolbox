package services

import (
	"log/slog"
	"sync"

	"github.com/bsv-blockchain/go-chaintracks/chaintracks"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/eventqueue"
)

// tipBroadcaster allows multiple subscribers to receive new tip events
type tipBroadcaster struct {
	mu          sync.RWMutex
	subscribers map[chan *chaintracks.BlockHeader]*eventqueue.Queue[*chaintracks.BlockHeader]
	logger      *slog.Logger
}

func newTipBroadcaster(logger *slog.Logger) *tipBroadcaster {
	return &tipBroadcaster{
		logger:      logger,
		subscribers: make(map[chan *chaintracks.BlockHeader]*eventqueue.Queue[*chaintracks.BlockHeader]),
	}
}

// Subscribe registers a user-provided channel to receive new tip events.
// Events are never dropped and the chaintracks event loop never waits for the
// subscriber: events the channel cannot take yet are buffered in memory and
// delivered in order. The caller owns the channel and may close it once the
// unsubscribe function has returned.
// Returns an unsubscribe function that removes the channel from the subscriber
// list and discards events not yet delivered to it.
func (t *tipBroadcaster) Subscribe(ch chan *chaintracks.BlockHeader) func() {
	t.mu.Lock()
	queue, ok := t.subscribers[ch]
	if !ok {
		queue = eventqueue.New(eventqueue.StreamTip, chan<- *chaintracks.BlockHeader(ch), t.logger)
		t.subscribers[ch] = queue
	}
	t.mu.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			t.mu.Lock()
			if t.subscribers[ch] == queue {
				delete(t.subscribers, ch)
			}
			t.mu.Unlock()

			// Returns once the queue stopped sending, so the caller may close ch.
			queue.Discard()
		})
	}
}

// broadcast hands the event to every subscriber's queue. It never blocks.
func (t *tipBroadcaster) broadcast(tip *chaintracks.BlockHeader) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	for _, queue := range t.subscribers {
		queue.Publish(tip)
	}
}
