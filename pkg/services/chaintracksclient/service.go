package chaintracksclient

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-chaintracks/chaintracks"
	"github.com/bsv-blockchain/go-chaintracks/config"
	"github.com/bsv-blockchain/go-sdk/chainhash"
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

func New(logger *slog.Logger, cfg *config.Config, p2pClient *p2p.Client, opts ...Option) (*Adapter, error) {
	logger = logging.Child(logger, "chaintracks")

	adapter := &Adapter{
		logger: logger,
	}

	for _, opt := range opts {
		opt(adapter)
	}

	if adapter.ct == nil {
		ct, err := cfg.Initialize(context.Background(), "wallet-toolbox", p2pClient)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize chaintracks: %w", err)
		}
		adapter.ct = ct
	}

	return adapter, nil
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

func (a *Adapter) CurrentHeight(ctx context.Context) (uint32, error) {
	ch, err := a.ct.CurrentHeight(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to find current height: %w", err)
	}

	return ch, nil
}

func (a *Adapter) IsValidRootForHeight(ctx context.Context, root *chainhash.Hash, height uint32) (bool, error) {
	isValid, err := a.ct.IsValidRootForHeight(ctx, root, height)
	if err != nil {
		return false, fmt.Errorf("failed to check valid root for height: %w", err)
	}

	return isValid, nil
}

// GetHeight returns the current blockchain height
func (a *Adapter) GetHeight(ctx context.Context) uint32 {
	return a.ct.GetHeight(ctx)
}

// GetTip returns the current chain tip
func (a *Adapter) GetTip(ctx context.Context) *chaintracks.BlockHeader {
	return a.ct.GetTip(ctx)
}

// GetHeaderByHeight retrieves a block header by its height
func (a *Adapter) GetHeaderByHeight(ctx context.Context, height uint32) (*chaintracks.BlockHeader, error) {
	header, err := a.ct.GetHeaderByHeight(ctx, height)
	if err != nil {
		return nil, fmt.Errorf("failed to get header by height %d: %w", height, err)
	}

	return header, nil
}

// GetHeaderByHash retrieves a block header by its hash
func (a *Adapter) GetHeaderByHash(ctx context.Context, hash *chainhash.Hash) (*chaintracks.BlockHeader, error) {
	header, err := a.ct.GetHeaderByHash(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("failed to get header by hash %s: %w", hash.String(), err)
	}

	return header, nil
}

// GetHeaders retrieves multiple headers starting from the given height
func (a *Adapter) GetHeaders(ctx context.Context, height, count uint32) ([]*chaintracks.BlockHeader, error) {
	headers, err := a.ct.GetHeaders(ctx, height, count)
	if err != nil {
		return nil, fmt.Errorf("failed to get headers from height %d (count %d): %w", height, count, err)
	}

	return headers, nil
}

// GetNetwork returns the network name (mainnet, testnet, etc.)
func (a *Adapter) GetNetwork(ctx context.Context) (string, error) {
	network, err := a.ct.GetNetwork(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get network: %w", err)
	}

	return network, nil
}
