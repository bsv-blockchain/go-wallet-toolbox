package tasks

import (
	"context"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type TaskInterface interface {
	Run(ctx context.Context) error
}

// StatusPublisher hands a transaction status event to the subscriber. Publish
// must never block and never drop the event (see eventqueue.Queue).
type StatusPublisher interface {
	Publish(msg wdk.CurrentTxStatus)
}
