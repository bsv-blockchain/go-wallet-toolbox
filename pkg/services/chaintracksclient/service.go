package chaintracksclient

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-chaintracks/chaintracks"
	"github.com/bsv-blockchain/go-chaintracks/config"
	p2p "github.com/bsv-blockchain/go-teranode-p2p-client"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
)

type Callbacks struct {
	OnTip func(*chaintracks.BlockHeader) error
}

type Adapter struct {
	logger *slog.Logger

	ct      chaintracks.Chaintracks
	tipChan <-chan *chaintracks.BlockHeader
	// TODO: reorgChan <- add when go-chaintract supports  reorg
}

func New(logger *slog.Logger, cfg *config.Config, p2pClient *p2p.Client) (*Adapter, error) {
	logger = logging.Child(logger, "chaintracks")

	ct, err := cfg.Initialize(context.Background(), "wallet-toolbox", p2pClient)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize chaintracks: %w", err)
	}

	return &Adapter{
		logger: logger,
		ct:     ct,
	}, nil
}

func (a *Adapter) Start(ctx context.Context, cb Callbacks) error {
	if cb.OnTip == nil {
		// TODO: warn for now but maybe we should error?
		a.logger.Warn("onTip function is nil, tipChan results will be ignored")
	}
	a.tipChan = a.ct.Subscribe(ctx)

	go func() {
		for header := range a.tipChan {
			if cb.OnTip != nil {
				if err := cb.OnTip(header); err != nil {
					a.logger.Error("onTip callback failed", "height", header.Height, "hash", header.Hash.String(), "err", err)
				}
			}
		}
	}()

	// TODO: a.reorgChan = a.ct.SubscribeReorg()... add when go-chaintracks supports reorg

	return nil
}
