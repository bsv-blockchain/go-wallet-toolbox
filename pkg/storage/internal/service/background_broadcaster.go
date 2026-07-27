package service

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/bsv-blockchain/go-sdk/transaction"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

const (
	// BackgroundBroadcasterWorkerCount is the default number of workers that
	// process broadcast items.
	//
	// Worker count is the ceiling on network acceptance for delayed broadcast,
	// and therefore on any throughput that depends on outputs becoming
	// spendable: an output is only marked spendable once its transaction is
	// accepted by the network (see actions.updateSingleTx). At ~300ms per post
	// this default sustains only ~33 tx/s, which a high-rate workload will
	// overflow — its queued transactions then fall back to the far slower
	// send_waiting cron and risk being aborted by the abandon sweep. Deployments
	// expecting that load raise it via Sizing (the throughput strategy does).
	//
	// The default stays small because every storage provider starts this pool
	// eagerly, including the many short-lived ones a test suite creates.
	BackgroundBroadcasterWorkerCount = 10

	// BackgroundBroadcasterChannelSize is the default buffer size for the
	// broadcast channel. Add() drops to the cron fallback once it is full.
	BackgroundBroadcasterChannelSize = 1000
)

// Sizing configures the broadcaster's capacity. Zero values select the
// defaults above.
type Sizing struct {
	Workers     int
	ChannelSize int
}

func (s Sizing) workers() int {
	if s.Workers <= 0 {
		return BackgroundBroadcasterWorkerCount
	}
	return s.Workers
}

func (s Sizing) channelSize() int {
	if s.ChannelSize <= 0 {
		return BackgroundBroadcasterChannelSize
	}
	return s.ChannelSize
}

type broadcaster interface {
	BackgroundBroadcast(ctx context.Context, beef *transaction.Beef, txIDs []string) ([]wdk.ReviewActionResult, error)
}

type BackgroundBroadcaster struct {
	sizing           Sizing
	ctx              context.Context
	cancel           context.CancelFunc
	broadcastChannel chan broadcastItem
	wg               sync.WaitGroup
	logger           *slog.Logger
	broadcastHandler broadcaster

	// optional notification channel
	txBroadcastedChannel chan<- wdk.CurrentTxStatus

	stopOnce sync.Once
}

type broadcastItem struct {
	beef  *transaction.Beef
	txIDs []string
}

func NewBackgroundBroadcaster(ctx context.Context, parentLogger *slog.Logger, broadcastHandler broadcaster, txBroadcastedChannel chan<- wdk.CurrentTxStatus, sizing Sizing) *BackgroundBroadcaster {
	bbContext, cancel := context.WithCancel(ctx)
	logger := logging.Child(parentLogger, "BackgroundBroadcaster")
	return &BackgroundBroadcaster{
		sizing:               sizing,
		ctx:                  bbContext,
		cancel:               cancel,
		broadcastChannel:     make(chan broadcastItem, sizing.channelSize()),
		logger:               logger,
		broadcastHandler:     broadcastHandler,
		txBroadcastedChannel: txBroadcastedChannel,
	}
}

func (bb *BackgroundBroadcaster) Start() {
	for i := 0; i < bb.sizing.workers(); i++ {
		bb.wg.Add(1)
		go bb.worker()
	}
}

func (bb *BackgroundBroadcaster) Stop() {
	bb.stopOnce.Do(func() {
		bb.cancel()
		bb.wg.Wait()
		close(bb.broadcastChannel)
	})
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
				bb.logger.ErrorContext(bb.ctx, "Failed to broadcast transaction", "error", err, "txIDs", item.txIDs)
			} else {
				bb.logger.InfoContext(bb.ctx, "Successfully broadcasted transaction", "txIDs", item.txIDs)
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
			Labels:    res.Labels,
		}

		if len(res.Errors) > 0 {
			broadcastError := &wdk.CurrentTxError{
				CompetingTxs: res.CompetingTxs,
				Errors:       map[string]error(res.Errors),
			}
			msg.Error = broadcastError
		}

		select {
		case bb.txBroadcastedChannel <- msg:
		case <-bb.ctx.Done():
			return fmt.Errorf("context done while sending tx status update: %w", bb.ctx.Err())
		default:
			bb.logger.WarnContext(bb.ctx, "TxBroadcasted channel in background broadcaster is full, dropping event")
		}
	}

	return nil
}
