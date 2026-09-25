// Package eventqueue delivers events to subscriber-provided channels without ever
// dropping them and without ever blocking the producer.
//
// A subscriber channel has a fixed buffer chosen by the subscriber, while bursts
// (e.g. a thousand transactions broadcast at once) can be far larger. Sending with
// select/default drops the overflow, and a blocking send stalls the producer (the
// background broadcaster, a monitor task, the chaintracks event loop). A Queue sits
// in between: Publish appends to an unbounded in-memory FIFO and returns at once,
// and a single forwarder goroutine per channel performs the blocking sends in
// publish order.
//
// The queue is in-memory only: events still buffered when the process dies are
// lost. It guarantees delivery for the lifetime of the process, not durability.
package eventqueue

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/metrics"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
)

// DefaultDrainTimeout bounds how long a shutting-down owner waits for the
// subscriber to read the backlog. The queue never gives up on its own while
// open; this deadline exists only so a subscriber that stopped reading cannot
// hang shutdown forever.
const DefaultDrainTimeout = 30 * time.Second

// Stream names used in logs and metrics.
const (
	StreamTxBroadcasted = "tx_broadcasted"
	StreamTxProven      = "tx_proven"
	StreamReorg         = "reorg"
	StreamTip           = "tip"
)

// warnEvery logs a "subscriber falling behind" warning every N buffered events.
const warnEvery = 1000

type entry[T any] struct {
	value T
	at    time.Time
}

// Queue is an unbounded, ordered mailbox feeding one subscriber channel.
// A nil *Queue is valid: Publish is a no-op and Close/Discard return 0.
type Queue[T any] struct {
	stream string
	out    chan<- T
	logger *slog.Logger

	mu     sync.Mutex
	buf    []entry[T] // FIFO: buf[0] is the next event to deliver
	closed bool       // set by Close/Discard: no more events are accepted

	wake  chan struct{} // capacity 1: "the buffer changed, look again"
	abort chan struct{} // closed to interrupt the forwarder, even mid-send
	done  chan struct{} // closed when the forwarder has exited

	abortOnce sync.Once
}

// New creates a queue feeding out and starts its forwarder goroutine.
// stream names the event stream in logs and metrics. A nil out yields a nil queue.
func New[T any](stream string, out chan<- T, logger *slog.Logger) *Queue[T] {
	if out == nil {
		return nil
	}

	q := &Queue[T]{
		stream: stream,
		out:    out,
		logger: logging.DefaultIfNil(logger).With(slog.String("stream", stream)),
		wake:   make(chan struct{}, 1),
		abort:  make(chan struct{}),
		done:   make(chan struct{}),
	}

	live.add(q)
	go q.forward()

	return q
}

// Publish appends v to the queue. It never blocks and never drops v while the
// queue is open. After Close or Discard the event is counted as undelivered.
func (q *Queue[T]) Publish(v T) {
	if q == nil {
		return
	}

	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		q.logger.WarnContext(context.Background(), "event published after the subscriber queue was closed, dropping it")
		metrics.RecordEventsUndelivered(context.Background(), q.stream, metrics.EventUndeliveredAfterClose, 1)
		return
	}
	q.buf = append(q.buf, entry[T]{value: v, at: time.Now()})
	depth := len(q.buf)
	q.mu.Unlock()

	if depth%warnEvery == 0 {
		q.logger.WarnContext(context.Background(), "subscriber is not keeping up with events, buffering them in memory",
			slog.Int("depth", depth))
	}

	q.poke()
}

// Close stops accepting events and waits until the subscriber has read every
// buffered event or ctx is done. It returns the number of events that were not
// delivered. When Close returns the forwarder has exited, so the owner may close
// the subscriber channel safely.
func (q *Queue[T]) Close(ctx context.Context) int {
	if q == nil {
		return 0
	}

	q.markClosed()
	q.poke() // an idle forwarder wakes up, finds the queue closed and exits

	select {
	case <-q.done:
	case <-ctx.Done():
		q.stopForwarder()
	}

	undelivered := q.clear()
	if undelivered > 0 {
		q.logger.ErrorContext(ctx, "subscriber did not read all events before shutdown, events were not delivered",
			slog.Int("undelivered", undelivered))
		metrics.RecordEventsUndelivered(ctx, q.stream, metrics.EventUndeliveredShutdownTimeout, undelivered)
	}
	return undelivered
}

// Discard stops accepting events and drops everything still buffered. It is
// meant for an explicit unsubscribe, where the subscriber no longer wants events.
// It returns the number of dropped events. When Discard returns the forwarder
// has exited, so the owner may close the subscriber channel safely.
func (q *Queue[T]) Discard() int {
	if q == nil {
		return 0
	}

	q.markClosed()
	q.stopForwarder()

	dropped := q.clear()
	if dropped > 0 {
		q.logger.WarnContext(context.Background(), "subscriber unsubscribed with events pending, dropping them",
			slog.Int("dropped", dropped))
		metrics.RecordEventsUndelivered(context.Background(), q.stream, metrics.EventUndeliveredUnsubscribed, dropped)
	}
	return dropped
}

// Stats reports the number of buffered events and the age of the oldest one.
func (q *Queue[T]) Stats() (depth int, oldestAge time.Duration) {
	if q == nil {
		return 0, 0
	}

	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.buf) == 0 {
		return 0, 0
	}
	return len(q.buf), time.Since(q.buf[0].at)
}

// forward is the only goroutine sending to out, which is what keeps the events
// in publish order and lets the owner close out once this has returned.
func (q *Queue[T]) forward() {
	defer close(q.done)

	for {
		next, ok, closed := q.next()
		if !ok {
			// Nothing to deliver: once closed, nothing can be published either.
			if closed {
				return
			}
			select {
			case <-q.wake:
			case <-q.abort:
				return
			}
			continue
		}

		select {
		case q.out <- next.value:
			q.pop()
		case <-q.abort:
			return
		}
	}
}

// next returns the event to deliver, whether there was one, and whether the
// queue is closed.
func (q *Queue[T]) next() (_ entry[T], ok bool, closed bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.buf) == 0 {
		return entry[T]{}, false, q.closed
	}
	return q.buf[0], true, q.closed
}

func (q *Queue[T]) pop() {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.buf) == 0 {
		return
	}
	q.buf[0] = entry[T]{} // release the event for GC
	q.buf = q.buf[1:]     // append reclaims the consumed prefix when it grows the slice
}

// poke tells the forwarder to look at the queue again. The buffered channel
// keeps it non-blocking: a pending wake-up already says everything this one would.
func (q *Queue[T]) poke() {
	select {
	case q.wake <- struct{}{}:
	default:
	}
}

func (q *Queue[T]) markClosed() {
	q.mu.Lock()
	q.closed = true
	q.mu.Unlock()
}

func (q *Queue[T]) stopForwarder() {
	q.abortOnce.Do(func() { close(q.abort) })
	<-q.done
}

// clear empties the buffer after the forwarder has exited and unregisters the
// queue from the live set. It returns how many events were left.
func (q *Queue[T]) clear() int {
	live.remove(q)

	q.mu.Lock()
	defer q.mu.Unlock()
	left := len(q.buf)
	q.buf = nil
	return left
}
