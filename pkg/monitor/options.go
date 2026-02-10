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
}

// DaemonEventOption defines a function type for setting DaemonEventOptions.
type DaemonEventOption func(*DaemonEventOptions)

func defaultDaemonEventOptions() *DaemonEventOptions {
	return &DaemonEventOptions{
		onTxBroadcasted: nil,
		onTxProven:      nil,
		onReorg:         nil,
		onTip:           nil,
	}
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
