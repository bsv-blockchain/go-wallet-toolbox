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
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// Callbacks defines the callback functions for chaintracks events
type Callbacks struct {
	// OnTip is called when a new block tip is received
	OnTip func(*chaintracks.BlockHeader) error
	// OnReorg is called when a blockchain reorganization occurs
	OnReorg func(*chaintracks.ReorgEvent) error
}

// Adapter provides a wrapper around the chaintracks client with event subscription capabilities
type Adapter struct {
	logger    *slog.Logger
	p2pClient *p2p.Client

	ct        chaintracks.Chaintracks
	tipChan   <-chan *chaintracks.BlockHeader
	reorgChan <-chan *chaintracks.ReorgEvent
}

// New creates a new chaintracks adapter with the given configuration and P2P client
func New(logger *slog.Logger, cfg *config.Config, opts ...Option) (*Adapter, error) {
	logger = logging.Child(logger, "chaintracks")

	adapter := &Adapter{
		logger: logger,
	}

	for _, opt := range opts {
		opt(adapter)
	}

	if adapter.ct == nil {
		ct, err := cfg.Initialize(context.Background(), "wallet-toolbox", adapter.p2pClient)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize chaintracks: %w", err)
		}
		adapter.ct = ct
	}

	return adapter, nil
}

// Start begins listening for chaintracks events and invokes the provided callbacks
func (a *Adapter) Start(ctx context.Context, cb Callbacks) error {
	a.subscribeToTipChan(ctx, cb.OnTip)
	a.subscribeToReorgChan(ctx, cb.OnReorg)

	return nil
}

// subscribeToTipChan subscribes to new block tip events and processes them with the provided callback
func (a *Adapter) subscribeToTipChan(ctx context.Context, cb func(*chaintracks.BlockHeader) error) {
	if cb == nil {
		// TODO: warn for now but maybe we should error?
		a.logger.Warn("onTip callback is nil, tipChan results will be ignored")
		return
	}
	a.tipChan = a.ct.Subscribe(ctx)
	go func() {
		for header := range a.tipChan {
			if cb != nil {
				if err := cb(header); err != nil {
					a.logger.Error("onTip callback failed", "height", header.Height, "hash", header.Hash.String(), "err", err)
				}
			}
		}
	}()
}

// subscribeToReorgChan subscribes to blockchain reorganization events and processes them with the provided callback
func (a *Adapter) subscribeToReorgChan(ctx context.Context, cb func(*chaintracks.ReorgEvent) error) {
	if cb == nil {
		// TODO: warn for now but maybe we should error?
		a.logger.Warn("onReorg callback is nil, reorgChan results will be ignored")
		return
	}

	a.reorgChan = a.ct.SubscribeReorg(ctx)
	go func() {
		for reorgEvent := range a.reorgChan {
			if cb != nil {
				if err := cb(reorgEvent); err != nil {
					a.logger.Error("onReorg callback failed",
						"depth", reorgEvent.Depth,
						"new tip hash", reorgEvent.NewTip.Hash.String(),
						"orphaned hashes", reorgEvent.OrphanedHashes,
						"err", err)
				}
			}
		}
	}()
}

// CurrentHeight returns the current blockchain height from chaintracks
func (a *Adapter) CurrentHeight(ctx context.Context) (uint32, error) {
	ch, err := a.ct.CurrentHeight(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to find current height: %w", err)
	}

	return ch, nil
}

// IsValidRootForHeight checks if the given merkle root is valid for the specified block height
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
func (a *Adapter) GetTip(ctx context.Context) (*wdk.ChainBlockHeader, error) {
	tip := a.ct.GetTip(ctx)

	return &wdk.ChainBlockHeader{
		ChainBaseBlockHeader: wdk.ChainBaseBlockHeader{
			Version:      uint32(tip.Version), //nolint:gosec
			PreviousHash: tip.PrevHash.String(),
			MerkleRoot:   tip.MerkleRoot.String(),
			Time:         tip.Timestamp,
			Bits:         tip.Bits,
			Nonce:        tip.Nonce,
		},
		Hash:   tip.Hash.String(),
		Height: uint(tip.Height),
	}, nil
}

// GetHeaderByHeight retrieves a block header by its height
func (a *Adapter) GetHeaderByHeight(ctx context.Context, height uint32) (*wdk.ChainBlockHeader, error) {
	header, err := a.ct.GetHeaderByHeight(ctx, height)
	if err != nil {
		return nil, fmt.Errorf("failed to get header by height %d: %w", height, err)
	}

	return &wdk.ChainBlockHeader{
		ChainBaseBlockHeader: wdk.ChainBaseBlockHeader{
			Version:      uint32(header.Version), //nolint:gosec
			PreviousHash: header.PrevHash.String(),
			MerkleRoot:   header.MerkleRoot.String(),
			Time:         header.Timestamp,
			Bits:         header.Bits,
			Nonce:        header.Nonce,
		},
		Hash:   header.Hash.String(),
		Height: uint(header.Height),
	}, nil
}

// GetHeaderByHash retrieves a block header by its hash
func (a *Adapter) GetHeaderByHash(ctx context.Context, hash string) (*wdk.ChainBlockHeader, error) {
	chainHash, err := chainhash.NewHashFromHex(hash)
	if err != nil {
		return nil, fmt.Errorf("failed to create chainhash from hash string: %s, err: %w", hash, err)
	}

	header, err := a.ct.GetHeaderByHash(ctx, chainHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get header by hash %s: %w", hash, err)
	}

	return &wdk.ChainBlockHeader{
		ChainBaseBlockHeader: wdk.ChainBaseBlockHeader{
			Version:      uint32(header.Version), //nolint:gosec
			PreviousHash: header.PrevHash.String(),
			MerkleRoot:   header.MerkleRoot.String(),
			Time:         header.Timestamp,
			Bits:         header.Bits,
			Nonce:        header.Nonce,
		},
		Hash:   header.Hash.String(),
		Height: uint(header.Height),
	}, nil
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
