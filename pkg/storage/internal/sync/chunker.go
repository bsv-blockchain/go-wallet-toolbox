package sync

import (
	"context"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type Chunker interface {
	Name() string
	MaxPageSize() uint64
	IsApplicable(requestedEntities OffsetsLookup) bool
	FirstPage(offsetsLookup OffsetsLookup) *queryopts.Paging
	Process(ctx context.Context, userID int, page *queryopts.Paging, since *time.Time, result *wdk.SyncChunk) (num uint64, err error)
}

func chunkerQueryOptions(page *queryopts.Paging, since *time.Time) []queryopts.Options {
	opts := []queryopts.Options{
		queryopts.WithPage(*page),
	}

	if since != nil {
		opts = append(opts, queryopts.WithSince(queryopts.Since{Time: *since, Field: "updated_at"}))
	}
	return opts
}
