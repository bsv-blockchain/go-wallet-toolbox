package tasks

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type WaitingTransactionsSender interface {
	SendWaitingTransactions(ctx context.Context, minTransactionAge time.Duration) (*wdk.ProcessActionResult, error)
}

type SendWaitingTask struct {
	storage              WaitingTransactionsSender
	firstRun             bool
	communicationChannel chan<- defs.MonitorTaskResponse
	logger               *slog.Logger
}

func NewSendWaitingTask(storage WaitingTransactionsSender, communicationChannel chan<- defs.MonitorTaskResponse, log *slog.Logger) TaskInterface {
	return &SendWaitingTask{
		storage:              storage,
		firstRun:             true,
		communicationChannel: communicationChannel,
		logger:               log,
	}
}

func (t *SendWaitingTask) Run(ctx context.Context) error {
	results, err := t.storage.SendWaitingTransactions(ctx, t.minTransactionAge())
	if err != nil {
		return fmt.Errorf("send waiting transactions failed: %w", err)
	}

	if t.communicationChannel == nil || results == nil {
		return nil
	}

	for _, res := range results.NotDelayedResults {
		msg := defs.MonitorTaskResponse{
			TxID:   res.TxID.String(),
			Status: "broadcasted",
		}

		if res.Status != wdk.ReviewActionResultStatusSuccess {
			msg.Status = "failed"
		}

		select {
		case t.communicationChannel <- msg:
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
