package sync

import (
	"context"
	"time"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
)

type Chunker interface {
	Name() string
	MaxDivider() uint64
	IsApplicable(requestedEntities OffsetsLookup) bool
	Process(ctx context.Context, userID int, limit, relativeOffset uint64, offsetsLookup OffsetsLookup, since *time.Time, result *wdk.SyncChunk) (num uint64, err error)
}
