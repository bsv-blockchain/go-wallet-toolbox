package service

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
)

const (
	BackgroundBroadcasterChannelSize = 1000
	BackgroundBroadcasterWorkerCount = 10
)

type broadcaster interface {
	BackgroundBroadcast(ctx context.Context, beef *transaction.Beef, txIDs []string) error
}

type BackgroundBroadcaster struct {
	ctx              context.Context
	cancel           context.CancelFunc
	broadcastChannel chan broadcastItem
	wg               sync.WaitGroup
	logger           *slog.Logger
	broadcastHandler broadcaster
}

type broadcastItem struct {
	beef  *transaction.Beef
	txIDs []string
}

func NewBackgroundBroadcaster(ctx context.Context, parentLogger *slog.Logger, broadcastHandler broadcaster) *BackgroundBroadcaster {
	bbContext, cancel := context.WithCancel(ctx)
	logger := logging.Child(parentLogger, "BackgroundBroadcaster")
	return &BackgroundBroadcaster{
		ctx:              bbContext,
		cancel:           cancel,
		broadcastChannel: make(chan broadcastItem, BackgroundBroadcasterChannelSize),
		logger:           logger,
		broadcastHandler: broadcastHandler,
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

	err = bb.broadcastHandler.BackgroundBroadcast(bb.ctx, item.beef, item.txIDs)
	if err != nil {
		return fmt.Errorf("failed to broadcast beef: %w", err)
	}

	return nil
}
