package tasks

import (
	"context"
	"fmt"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type WaitingTransactionsSender interface {
	SendWaitingTransactions(ctx context.Context, minTransactionAge time.Duration) (*wdk.ProcessActionResult, error)
}

type SendWaitingTask struct {
	storage             WaitingTransactionsSender
	firstRun            bool
	txBroadcastedEvents StatusPublisher
}

func NewSendWaitingTask(storage WaitingTransactionsSender, txBroadcastedEvents StatusPublisher) TaskInterface {
	return &SendWaitingTask{
		storage:             storage,
		firstRun:            true,
		txBroadcastedEvents: txBroadcastedEvents,
	}
}

func (t *SendWaitingTask) Run(ctx context.Context) error {
	results, err := t.storage.SendWaitingTransactions(ctx, t.minTransactionAge())
	if err != nil {
		return fmt.Errorf("send waiting transactions failed: %w", err)
	}

	if t.txBroadcastedEvents == nil || results == nil {
		return nil
	}

	for _, res := range results.NotDelayedResults {
		msg := wdk.CurrentTxStatus{
			TxID:      res.TxID.String(),
			Status:    res.Status.ToStandardizedStatus(),
			Reference: res.Reference,
			Labels:    res.Labels,
		}

		if len(res.Errors) > 0 {
			broadcastError := &wdk.CurrentTxError{
				CompetingTxs: res.CompetingTxs,
				Errors:       map[string]error(res.Errors),
			}
			msg.Error = broadcastError
		}

		t.txBroadcastedEvents.Publish(msg)
	}

	return nil
}

func (t *SendWaitingTask) minTransactionAge() time.Duration {
	if t.firstRun {
		t.firstRun = false
		return 0
	}
	return 5 * time.Minute
}
