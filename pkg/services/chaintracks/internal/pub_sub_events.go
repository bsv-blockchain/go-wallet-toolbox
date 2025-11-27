package internal

import (
	"log/slog"
	"sync"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
)

type PubSubEvents[T any] struct {
	logger      *slog.Logger
	subscribers map[chan T]struct{}
	mutex       sync.RWMutex
}

func NewPubSubEvents[T any](logger *slog.Logger) *PubSubEvents[T] {
	return &PubSubEvents[T]{
		logger:      logging.Child(logger, "pub_sub_events"),
		subscribers: make(map[chan T]struct{}),
	}
}

func (s *PubSubEvents[T]) Subscribe() (readOnlyChan <-chan T, unsubscribe func()) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Create a buffered channel to prevent immediate blocking
	ch := make(chan T, 10)
	s.subscribers[ch] = struct{}{}

	readOnlyChan = ch
	unsubscribe = func() {
		s.mutex.Lock()
		defer s.mutex.Unlock()

		delete(s.subscribers, ch)
		close(ch)
	}
	return
}

func (s *PubSubEvents[T]) Publish(event T) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	for ch := range s.subscribers {
		// Non-blocking send to prevent blocking if the channel is full
		select {
		case ch <- event:
		default:
		}
	}
}
