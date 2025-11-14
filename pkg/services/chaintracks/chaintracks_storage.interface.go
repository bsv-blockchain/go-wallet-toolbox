package chaintracks

import (
	"context"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracks/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// Storage defines an interface for storage backends capable of storing chaintracks data.
type Storage interface {
	Migrate(ctx context.Context) error

	Query(ctx context.Context) models.StorageQueries
}

// LiveIngestor defines a contract for streaming and retrieving live blockchain block header data.
// It provides methods for looking up headers by hash and for receiving header events in real-time.
type LiveIngestor interface {
	StartListening(parentCtx context.Context, respChan chan wdk.ChainBlockHeader)
	StopListening()

	GetHeaderByHash(ctx context.Context, hash string) (*wdk.ChainBlockHeader, error)
	GetPresentHeight(ctx context.Context) (uint, error)
}

// NamedLiveIngestor associates a human-readable name with a LiveIngestor implementation for identification purposes.
// It enables grouping or distinguishing multiple live realtime blockchain data ingestors within services.
type NamedLiveIngestor struct {
	Name     string
	Ingestor LiveIngestor
}

// BulkIngestor defines an interface for bulk synchronization of block headers within specified height ranges.
// The Synchronize method ingests headers up to the given presentHeight for provided height ranges and returns insertion results.
// TODO: refine return type from 'any' to a more specific type representing synchronization results.
type BulkIngestor interface {
	Synchronize(ctx context.Context, presentHeight uint, ranges models.HeightRanges) (any, error)
}

// NamedBulkIngestor associates a descriptive name with a BulkIngestor interface for organized bulk header synchronization tasks.
type NamedBulkIngestor struct {
	Name     string
	Ingestor BulkIngestor
}
