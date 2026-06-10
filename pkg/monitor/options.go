package monitor

import (
	"github.com/bsv-blockchain/go-chaintracks/chaintracks"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// DaemonEventOptions holds options for communication channels used by the monitor daemon.
type DaemonEventOptions struct {
	onTxBroadcasted chan<- wdk.CurrentTxStatus
	onTxProven      chan<- wdk.CurrentTxStatus

	onReorg <-chan *chaintracks.ReorgEvent
	onTip   <-chan *chaintracks.BlockHeader

	broadcastEventStreamer BroadcastEventStreamer
}

// DaemonEventOption defines a function type for setting DaemonEventOptions.
type DaemonEventOption func(*DaemonEventOptions)

func defaultDaemonEventOptions() *DaemonEventOptions {
	return &DaemonEventOptions{
		onTxBroadcasted:        nil,
		onTxProven:             nil,
		onReorg:                nil,
		onTip:                  nil,
		broadcastEventStreamer: nil,
	}
}

// DefaultDaemonEventOptions builds a *DaemonEventOptions by applying the
// provided option functions.  Exported so tests outside the package can
// construct a configured options value without going through a GORM locker.
func DefaultDaemonEventOptions(opts ...DaemonEventOption) *DaemonEventOptions {
	o := defaultDaemonEventOptions()
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// WithBroadcastedTxChannel sets the channel for broadcasted transaction notifications.
func WithBroadcastedTxChannel(ch chan<- wdk.CurrentTxStatus) func(*DaemonEventOptions) {
	return func(o *DaemonEventOptions) {
		o.onTxBroadcasted = ch
	}
}

// WithProvenTxChannel sets the channel for proven transaction notifications.
func WithProvenTxChannel(ch chan<- wdk.CurrentTxStatus) func(*DaemonEventOptions) {
	return func(o *DaemonEventOptions) {
		o.onTxProven = ch
	}
}

// WithReorgChannel sets the channel for receiving reorg events from chaintracks.
//
// NOTE: This is typically not used directly by users. When using infra.Server,
// this is automatically wired to chaintracks. Only use this if you are manually
// setting up the monitor.
func WithReorgChannel(ch <-chan *chaintracks.ReorgEvent) func(*DaemonEventOptions) {
	return func(o *DaemonEventOptions) {
		o.onReorg = ch
	}
}

// WithTipChannel sets the channel for receiving new tips events from chaintracks.
//
// NOTE: This is typically not used directly by users. When using infra.Server,
// this is automatically wired to chaintracks. Only use this if you are manually
// setting up the monitor.
func WithTipChannel(ch <-chan *chaintracks.BlockHeader) func(*DaemonEventOptions) {
	return func(o *DaemonEventOptions) {
		o.onTip = ch
	}
}

// WithBroadcastEventStream registers a BroadcastEventStreamer whose SSE events
// are consumed by the monitor daemon.  When set, Daemon.Start launches a
// dedicated goroutine that reads events and calls
// MonitoredStorage.ProcessExternalTxStatusUpdate, persisting a replay cursor
// in the key-value store so the stream can be resumed after a restart.
func WithBroadcastEventStream(streamer BroadcastEventStreamer) DaemonEventOption {
	return func(o *DaemonEventOptions) {
		o.broadcastEventStreamer = streamer
	}
}
