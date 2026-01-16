package monitor

import (
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
)

type DaemonCommunicationOptions struct {
	onTxBroadcasted chan<- defs.MonitorTaskResponse
	onTxProven      chan<- defs.MonitorTaskResponse
}

type CommunicationOption func(*DaemonCommunicationOptions)

func defaultDaemonCommunicationOptions() *DaemonCommunicationOptions {
	return &DaemonCommunicationOptions{
		onTxBroadcasted: nil,
		onTxProven:      nil,
	}
}

func WithBroadcastedTxChannel(ch chan<- defs.MonitorTaskResponse) func(*DaemonCommunicationOptions) {
	return func(o *DaemonCommunicationOptions) {
		fmt.Println("Setting broadcasted tx channel")
		o.onTxBroadcasted = ch
	}
}

func WithProvenTxChannel(ch chan<- defs.MonitorTaskResponse) func(*DaemonCommunicationOptions) {
	return func(o *DaemonCommunicationOptions) {
		fmt.Println("Setting proven tx channel")
		o.onTxProven = ch
	}
}
