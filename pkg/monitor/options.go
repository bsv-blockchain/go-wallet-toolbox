package monitor

import (
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
)

// DaemonEventOptions holds options for communication channels used by the monitor daemon.
type DaemonEventOptions struct {
	onTxBroadcasted chan<- defs.TransactionStatusUpdate
	onTxProven      chan<- defs.TransactionStatusUpdate
}

// DaemonEventOption defines a function type for setting DaemonEventOptions.
type DaemonEventOption func(*DaemonEventOptions)

func defaultDaemonEventOptions() *DaemonEventOptions {
	return &DaemonEventOptions{
		onTxBroadcasted: nil,
		onTxProven:      nil,
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
