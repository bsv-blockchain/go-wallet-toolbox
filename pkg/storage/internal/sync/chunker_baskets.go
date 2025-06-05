package sync

import (
	"context"
	"fmt"
	"time"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/must"
)

type chunkerBaskets struct {
	repo Repository
}

func newChunkerBaskets(repo Repository) *chunkerBaskets {
	return &chunkerBaskets{
		repo: repo,
	}
}

func (c *chunkerBaskets) Name() string {
	return "baskets"
}

func (c *chunkerBaskets) MaxDivider() uint64 {
	return 1
}

func (c *chunkerBaskets) IsApplicable(requestedEntities OffsetsLookup) bool {
	_, ok := requestedEntities[wdk.OutputBasketEntityName]
	return ok
}

func (c *chunkerBaskets) Process(ctx context.Context, userID int, limit, relativeOffset uint64, offsetsLookup OffsetsLookup, since *time.Time, result *wdk.SyncChunk) (num uint64, err error) {
	offset := offsetsLookup[wdk.OutputBasketEntityName] + relativeOffset

	opts := []queryopts.QueryOptsUnion{
		queryopts.WithPage(queryopts.Paging{
			Limit:  must.ConvertToIntFromUnsigned(limit),
			Offset: must.ConvertToIntFromUnsigned(offset),
		}),
	}

	if since != nil {
		opts = append(opts, queryopts.WithSince(queryopts.Since{Time: *since}))
	}

	rows, err := c.repo.FindBasketsByUserID(ctx, userID, opts...)
	if err != nil {
		return 0, fmt.Errorf("fetch baskets by user id: %w", err)
	}

	result.OutputBaskets = append(result.OutputBaskets, rows...)

	return must.ConvertToUInt64(len(rows)), nil
}
