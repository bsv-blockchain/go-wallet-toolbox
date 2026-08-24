package defs

import "fmt"

const (
	// MaxBroadcasterWorkers bounds the delayed-broadcast worker pool. Past this
	// point the database connection pool and the broadcast service, not the
	// number of goroutines, decide throughput.
	MaxBroadcasterWorkers = 1024

	// MaxBroadcasterChannelSize bounds the delayed-broadcast queue. Every queued
	// item holds a parsed BEEF - a structure, not raw bytes - so an oversized
	// queue is an out-of-memory risk rather than a harmless buffer. The cap is
	// here to make a typo cost a config error instead of a dead process.
	MaxBroadcasterChannelSize = 100_000
)

// BackgroundBroadcaster sizes the delayed-broadcast queue that createAction
// hands its transactions to.
//
// Both fields are optional and a zero means "not configured": the sizing is then
// derived from the throughput strategy when it is enabled, and otherwise falls
// back to the package defaults. Because the fields resolve independently, an
// operator can widen the queue for bursty traffic without touching the worker
// count, or the other way round.
type BackgroundBroadcaster struct {
	// Workers is the number of concurrent posts. Throughput is roughly
	// Workers / (time of one post), so this is the ceiling on how fast queued
	// transactions reach the network.
	Workers uint `mapstructure:"workers"`
	// ChannelSize is how many transactions may wait for a worker. It absorbs
	// bursts; it does not raise sustained throughput. Whatever does not fit is
	// deferred to the send_waiting cron, which drains far more slowly.
	ChannelSize uint `mapstructure:"channel_size"`
}

// DefaultBackgroundBroadcaster returns the unconfigured value, which preserves
// the sizing every existing deployment gets today.
func DefaultBackgroundBroadcaster() BackgroundBroadcaster {
	return BackgroundBroadcaster{}
}

// Validate verifies the background broadcaster configuration.
func (b *BackgroundBroadcaster) Validate() error {
	if b.Workers > MaxBroadcasterWorkers {
		return fmt.Errorf("workers must not exceed %d, got %d", MaxBroadcasterWorkers, b.Workers)
	}
	if b.ChannelSize > MaxBroadcasterChannelSize {
		return fmt.Errorf("channel_size must not exceed %d, got %d", MaxBroadcasterChannelSize, b.ChannelSize)
	}
	return nil
}
