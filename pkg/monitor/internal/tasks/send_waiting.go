package tasks

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type WaitingTransactionsSender interface {
	SendWaitingTransactions(ctx context.Context, minTransactionAge time.Duration) (*wdk.ProcessActionResult, error)
}

type SendWaitingTask struct {
	storage              WaitingTransactionsSender
	firstRun             bool
	txBroadcastedChannel chan<- wdk.CurrentTxStatus
	logger               *slog.Logger
}

func NewSendWaitingTask(storage WaitingTransactionsSender, txBroadcastedChannel chan<- wdk.CurrentTxStatus, log *slog.Logger) TaskInterface {
	return &SendWaitingTask{
		storage:              storage,
		firstRun:             true,
		txBroadcastedChannel: txBroadcastedChannel,
		logger:               log,
	}
}

func (t *SendWaitingTask) Run(ctx context.Context) error {
	results, err := t.storage.SendWaitingTransactions(ctx, t.minTransactionAge())
	if err != nil {
		return fmt.Errorf("send waiting transactions failed: %w", err)
	}

	if t.txBroadcastedChannel == nil || results == nil {
		return nil
	}

	for _, res := range results.NotDelayedResults {
		msg := wdk.CurrentTxStatus{
			TxID:      res.TxID.String(),
			Status:    res.Status.ToStandardizedStatus(),
			Reference: res.Reference,
		}

		if len(res.Errors) > 0 {
			broadcastError := &wdk.CurrentTxError{
				CompetingTxs: res.CompetingTxs,
				Errors:       res.Errors,
			}
			msg.Error = broadcastError
		}

		select {
		case t.txBroadcastedChannel <- msg:
		case <-ctx.Done():
			return fmt.Errorf("context done while sending tx status update: %w", ctx.Err())
		default:
			t.logger.Warn("TxBroadcasted channel full, dropping event")
		}
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
