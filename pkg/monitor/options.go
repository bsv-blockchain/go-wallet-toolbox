package monitor

import (
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
)

type DaemonCommunicationOptions struct {
	onTxBroadcasted chan<- defs.MonitorTaskResponse
	onTxProven      chan<- defs.MonitorTaskResponse
}

type DaemonCommunicationOption func(*DaemonCommunicationOptions)

func defaultDaemonCommunicationOptions() *DaemonCommunicationOptions {
	return &DaemonCommunicationOptions{
		onTxBroadcasted: nil,
		onTxProven:      nil,
	}
}

func WithBroadcastedTxChannel(ch chan<- defs.MonitorTaskResponse) func(*DaemonCommunicationOptions) {
	return func(o *DaemonCommunicationOptions) {
		o.onTxBroadcasted = ch
	}
}

func WithProvenTxChannel(ch chan<- defs.MonitorTaskResponse) func(*DaemonCommunicationOptions) {
	return func(o *DaemonCommunicationOptions) {
		o.onTxProven = ch
	}
}
