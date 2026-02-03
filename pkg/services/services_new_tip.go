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

// Subscribe returns a channel that receives new tip events and an unsubscribe function.
// The channel is buffered to avoid blocking the broadcaster.
// Call the returned unsubscribe function to stop receiving events and close the channel.
func (t *tipBroadcaster) Subscribe() (<-chan *chaintracks.BlockHeader, func()) {
	ch := make(chan *chaintracks.BlockHeader, 10)

	t.mu.Lock()
	t.subscribers[ch] = struct{}{}
	t.mu.Unlock()

	unsubscribe := func() {
		t.mu.Lock()
		delete(t.subscribers, ch)
		close(ch)
		t.mu.Unlock()
	}

	return ch, unsubscribe
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
