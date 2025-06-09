package sync

import (
	"context"
	"time"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
)

type Chunker interface {
	Name() string
	MaxDivider() uint64
	IsApplicable(requestedEntities OffsetsLookup) bool
	FirstPage(offsetsLookup OffsetsLookup) *queryopts.Paging
	Process(ctx context.Context, userID int, page *queryopts.Paging, since *time.Time, result *wdk.SyncChunk) (num uint64, err error)
}
