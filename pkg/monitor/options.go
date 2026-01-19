package monitor

import (
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
)

// DaemonCommunicationOptions holds options for communication channels used by the monitor daemon.
type DaemonCommunicationOptions struct {
	onTxBroadcasted chan<- defs.MonitorTaskResponse
	onTxProven      chan<- defs.MonitorTaskResponse
}

// DaemonCommunicationOption defines a function type for setting DaemonCommunicationOptions.
type DaemonCommunicationOption func(*DaemonCommunicationOptions)

func defaultDaemonCommunicationOptions() *DaemonCommunicationOptions {
	return &DaemonCommunicationOptions{
		onTxBroadcasted: nil,
		onTxProven:      nil,
	}
}

// WithBroadcastedTxChannel sets the channel for broadcasted transaction notifications.
func WithBroadcastedTxChannel(ch chan<- defs.MonitorTaskResponse) func(*DaemonCommunicationOptions) {
	return func(o *DaemonCommunicationOptions) {
		o.onTxBroadcasted = ch
	}
}

// WithProvenTxChannel sets the channel for proven transaction notifications.
func WithProvenTxChannel(ch chan<- defs.MonitorTaskResponse) func(*DaemonCommunicationOptions) {
	return func(o *DaemonCommunicationOptions) {
		o.onTxProven = ch
	}
}
