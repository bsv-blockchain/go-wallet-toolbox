package monitor

import (
	"github.com/bsv-blockchain/go-chaintracks/chaintracks"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
)

// DaemonEventOptions holds options for communication channels used by the monitor daemon.
type DaemonEventOptions struct {
	onTxBroadcasted chan<- defs.TransactionStatusUpdate
	onTxProven      chan<- defs.TransactionStatusUpdate

	onReorg <-chan *chaintracks.ReorgEvent
}

// DaemonEventOption defines a function type for setting DaemonCommunicationOptions.
type DaemonEventOption func(*DaemonEventOptions)

func defaultDaemonEventOptions() *DaemonEventOptions {
	return &DaemonEventOptions{
		onTxBroadcasted: nil,
		onTxProven:      nil,
		onReorg:         nil,
	}
}

// WithBroadcastedTxChannel sets the channel for broadcasted transaction notifications.
func WithBroadcastedTxChannel(ch chan<- defs.TransactionStatusUpdate) func(*DaemonEventOptions) {
	return func(o *DaemonEventOptions) {
		o.onTxBroadcasted = ch
	}
}

// WithProvenTxChannel sets the channel for proven transaction notifications.
func WithProvenTxChannel(ch chan<- defs.TransactionStatusUpdate) func(*DaemonEventOptions) {
	return func(o *DaemonEventOptions) {
		o.onTxProven = ch
	}
}

// WithReorgChannel sets the channel for receiving reorg events.
func WithReorgChannel(ch <-chan *chaintracks.ReorgEvent) func(*DaemonEventOptions) {
	return func(o *DaemonEventOptions) {
		o.onReorg = ch
	}
}
