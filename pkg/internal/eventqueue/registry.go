package eventqueue

import (
	"context"
	"log/slog"
	"sync"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/metrics"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
)

// statser is the type-erased view of a Queue used for metrics.
type statser interface {
	stats() metrics.EventQueueStat
}

func (q *Queue[T]) stats() metrics.EventQueueStat {
	depth, oldest := q.Stats()
	return metrics.EventQueueStat{Stream: q.stream, Depth: depth, OldestAge: oldest}
}

// liveSet tracks open queues so a single metrics callback can report them all.
type liveSet struct {
	mu       sync.Mutex
	queues   map[statser]struct{}
	register sync.Once
}

var live = &liveSet{queues: map[statser]struct{}{}}

func (l *liveSet) add(q statser) {
	l.register.Do(func() {
		// The gauges live for the whole process; with no MeterProvider set the
		// callback never fires.
		if _, err := metrics.RegisterEventQueueGauges(l.snapshot); err != nil {
			logging.DefaultIfNil(nil).WarnContext(context.Background(), "failed to register event queue gauges", logging.Error(err))
		}
	})

	l.mu.Lock()
	defer l.mu.Unlock()
	l.queues[q] = struct{}{}
}

func (l *liveSet) remove(q statser) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.queues, q)
}

func (l *liveSet) snapshot() []metrics.EventQueueStat {
	l.mu.Lock()
	queues := make([]statser, 0, len(l.queues))
	for q := range l.queues {
		queues = append(queues, q)
	}
	l.mu.Unlock()

	stats := make([]metrics.EventQueueStat, 0, len(queues))
	for _, q := range queues {
		stats = append(stats, q.stats())
	}
	return stats
}

// shared is a queue together with the number of owners holding it.
type shared struct {
	queue any // *Queue[T]
	refs  int
}

var (
	sharedMu sync.Mutex
	sharedBy = map[any]*shared{} // keyed by the subscriber channel
)

// Acquire returns the queue feeding out, creating it on first use. Components
// that were handed the same subscriber channel (e.g. the storage background
// broadcaster and the monitor, both emitting "tx broadcasted") share one queue,
// so the channel still has exactly one sender and a single global event order.
//
// release must be called once by every owner when it stops publishing. The last
// release closes the queue, waiting for the subscriber to read the backlog until
// ctx is done. After the last release returns nothing sends to out anymore.
//
// A nil out yields a nil queue (Publish is a no-op) and a no-op release.
func Acquire[T any](stream string, out chan<- T, logger *slog.Logger) (*Queue[T], func(ctx context.Context)) {
	if out == nil {
		return nil, func(context.Context) {}
	}

	sharedMu.Lock()
	defer sharedMu.Unlock()

	s, ok := sharedBy[out]
	if !ok {
		s = &shared{queue: New(stream, out, logger)}
		sharedBy[out] = s
	}
	s.refs++

	q := s.queue.(*Queue[T]) //nolint:forcetypeassert // keyed by chan<- T, so the queue is always *Queue[T]

	var once sync.Once
	release := func(ctx context.Context) {
		once.Do(func() {
			sharedMu.Lock()
			s.refs--
			last := s.refs == 0
			if last {
				delete(sharedBy, out)
			}
			sharedMu.Unlock()

			if last {
				q.Close(ctx)
			}
		})
	}

	return q, release
}

// ReleaseWithDefaultTimeout calls release with a DefaultDrainTimeout deadline,
// for owners whose shutdown path has no context of its own.
func ReleaseWithDefaultTimeout(release func(ctx context.Context)) {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultDrainTimeout)
	defer cancel()
	release(ctx)
}
