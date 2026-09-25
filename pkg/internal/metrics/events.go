package metrics

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const eventsMeterName = "github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/eventqueue"

// Reasons an event could not be handed to a subscriber channel.
const (
	// EventUndeliveredShutdownTimeout: the owner stopped and the subscriber did
	// not drain the backlog before the shutdown deadline.
	EventUndeliveredShutdownTimeout = "shutdown_timeout"
	// EventUndeliveredUnsubscribed: the subscriber unsubscribed with events pending.
	EventUndeliveredUnsubscribed = "unsubscribed"
	// EventUndeliveredAfterClose: an event was published after the queue was closed.
	EventUndeliveredAfterClose = "after_close"
)

var (
	eventsInitOnce sync.Once

	eventsUndelivered metric.Int64Counter
)

func ensureEventsInstruments() {
	eventsInitOnce.Do(func() {
		meter := otel.Meter(eventsMeterName)
		// Instrument creation only fails on invalid names; fall back to no-op
		// instruments rather than propagating an error into the publish path.
		eventsUndelivered, _ = meter.Int64Counter("wallet.events.undelivered",
			metric.WithDescription("Events that never reached the subscriber channel, by stream and reason"))
	})
}

// RecordEventsUndelivered counts events of the stream that were not delivered
// to the subscriber channel. Any non-zero value means a subscriber missed events.
func RecordEventsUndelivered(ctx context.Context, stream, reason string, count int) {
	ensureEventsInstruments()
	if eventsUndelivered != nil && count > 0 {
		eventsUndelivered.Add(ctx, int64(count), metric.WithAttributes(
			attribute.String("stream", stream),
			attribute.String("reason", reason),
		))
	}
}

// EventQueueStat is a snapshot of one subscriber queue.
type EventQueueStat struct {
	Stream    string
	Depth     int
	OldestAge time.Duration
}

// EventQueueStatsFunc reports the current state of every live subscriber queue.
// It runs once per metrics export interval, never on the publish path.
type EventQueueStatsFunc func() []EventQueueStat

// RegisterEventQueueGauges registers the subscriber-queue gauges backed by stats.
// It returns an unregister func. With no MeterProvider set the callback never fires.
//
// Depth and oldest age show a subscriber falling behind: events are buffered, not
// lost, but memory grows and delivery latency rises until it catches up.
func RegisterEventQueueGauges(stats EventQueueStatsFunc) (func(), error) {
	meter := otel.Meter(eventsMeterName)

	depth, err := meter.Int64ObservableGauge("wallet.events.queue_depth",
		metric.WithDescription("Events buffered for a subscriber channel, waiting to be delivered"))
	if err != nil {
		return nil, fmt.Errorf("failed to create events.queue_depth gauge: %w", err)
	}
	oldest, err := meter.Float64ObservableGauge("wallet.events.queue_oldest_age_seconds",
		metric.WithUnit("s"),
		metric.WithDescription("Age of the oldest event still waiting for a subscriber channel"))
	if err != nil {
		return nil, fmt.Errorf("failed to create events.queue_oldest_age_seconds gauge: %w", err)
	}

	registration, err := meter.RegisterCallback(func(_ context.Context, observer metric.Observer) error {
		// Several queues may serve the same stream (e.g. one per reorg subscriber):
		// report the total depth and the worst age per stream.
		type agg struct {
			depth  int64
			oldest time.Duration
		}
		byStream := map[string]*agg{}
		for _, s := range stats() {
			a, ok := byStream[s.Stream]
			if !ok {
				a = &agg{}
				byStream[s.Stream] = a
			}
			a.depth += int64(s.Depth)
			a.oldest = max(a.oldest, s.OldestAge)
		}
		for stream, a := range byStream {
			attrs := metric.WithAttributes(attribute.String("stream", stream))
			observer.ObserveInt64(depth, a.depth, attrs)
			observer.ObserveFloat64(oldest, a.oldest.Seconds(), attrs)
		}
		return nil
	}, depth, oldest)
	if err != nil {
		return nil, fmt.Errorf("failed to register event queue gauge callback: %w", err)
	}

	return func() { _ = registration.Unregister() }, nil
}
