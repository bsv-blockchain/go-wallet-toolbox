package service

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

const (
	// BackgroundBroadcasterWorkerCount defines the number of workers that will process broadcast items.
	BackgroundBroadcasterWorkerCount = 10

	// BackgroundBroadcasterChannelSize defines the buffer size for the broadcast channel.
	// The average broadcast time in tests is 300ms, so for 10 workers, 1000 elements should be processed in 30sec
	// This is chosen as a trade-off between memory usage and throughput; it can be tuned based on expected workload.
	BackgroundBroadcasterChannelSize = 1000
)

type broadcaster interface {
	BackgroundBroadcast(ctx context.Context, beef *transaction.Beef, txIDs []string) ([]wdk.ReviewActionResult, error)
}

type BackgroundBroadcaster struct {
	ctx              context.Context
	cancel           context.CancelFunc
	broadcastChannel chan broadcastItem
	wg               sync.WaitGroup
	logger           *slog.Logger
	broadcastHandler broadcaster

	// optional notification channel
	txBroadcastedChannel chan<- wdk.CurrentTxStatus
}

type broadcastItem struct {
	beef  *transaction.Beef
	txIDs []string
}

func NewBackgroundBroadcaster(ctx context.Context, parentLogger *slog.Logger, broadcastHandler broadcaster, txBroadcastedChannel chan<- wdk.CurrentTxStatus) *BackgroundBroadcaster {
	bbContext, cancel := context.WithCancel(ctx)
	logger := logging.Child(parentLogger, "BackgroundBroadcaster")
	return &BackgroundBroadcaster{
		ctx:                  bbContext,
		cancel:               cancel,
		broadcastChannel:     make(chan broadcastItem, BackgroundBroadcasterChannelSize),
		logger:               logger,
		broadcastHandler:     broadcastHandler,
		txBroadcastedChannel: txBroadcastedChannel,
	}
}

func (bb *BackgroundBroadcaster) Start() {
	for i := 0; i < BackgroundBroadcasterWorkerCount; i++ {
		bb.wg.Add(1)
		go bb.worker()
	}
}

func (bb *BackgroundBroadcaster) Stop() {
	bb.cancel()
	bb.wg.Wait()
	close(bb.broadcastChannel)
}

func (bb *BackgroundBroadcaster) Add(beef *transaction.Beef, txIDs []string) (added bool) {
	bb.logger.InfoContext(bb.ctx, "Adding new beef to delayed broadcast", "txIDs", txIDs)
	select {
	case bb.broadcastChannel <- broadcastItem{beef: beef, txIDs: txIDs}:
		return true
	default:
		return false
	}
}

func (bb *BackgroundBroadcaster) worker() {
	defer bb.wg.Done()

	for {
		select {
		case <-bb.ctx.Done():
			return
		case item, ok := <-bb.broadcastChannel:
			if !ok {
				return
			}
			if err := bb.broadcast(&item); err != nil {
				bb.logger.Error("Failed to broadcast transaction", "error", err, "txIDs", item.txIDs)
			} else {
				bb.logger.Info("Successfully broadcasted transaction", "txIDs", item.txIDs)
			}
		}
	}
}

func (bb *BackgroundBroadcaster) broadcast(item *broadcastItem) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("recovered from panic during broadcast: %v", r)
		}
	}()

	results, err := bb.broadcastHandler.BackgroundBroadcast(bb.ctx, item.beef, item.txIDs)
	if err != nil {
		return fmt.Errorf("failed to broadcast beef: %w", err)
	}

	if bb.txBroadcastedChannel == nil || results == nil {
		return nil
	}

	for _, res := range results {
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
		case bb.txBroadcastedChannel <- msg:
		case <-bb.ctx.Done():
			return fmt.Errorf("context done while sending tx status update: %w", bb.ctx.Err())
		default:
			bb.logger.Warn("TxBroadcasted channel in background broadcaster is full, dropping event")
		}
	}

	return nil
}
