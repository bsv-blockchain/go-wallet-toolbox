package sync

import (
	"context"
	"time"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
)

type Chunker interface {
	Name() string
	MaxDivider() int
	IsApplicable(requestedEntities OffsetsLookup) bool
	Process(ctx context.Context, userID, limit, relativeOffset int, offsetsLookup OffsetsLookup, since *time.Time, result *wdk.SyncChunk) (num int, err error)
}
