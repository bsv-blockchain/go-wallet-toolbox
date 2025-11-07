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

type BulkIngestor interface {
	Synchronize(ctx context.Context, presentHeight uint, ranges models.HeightRanges) (*models.InsertHeaderResult, error)
}

type NamedBulkIngestor struct {
	Name     string
	Ingestor BulkIngestor
}
