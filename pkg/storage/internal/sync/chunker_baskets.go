package sync

import (
	"context"
	"fmt"
	"time"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
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

func (c *chunkerBaskets) MaxDivider() int {
	return 1
}

func (c *chunkerBaskets) IsApplicable(requestedEntities OffsetsLookup) bool {
	_, ok := requestedEntities[wdk.OutputBasketEntityName]
	return ok
}

func (c *chunkerBaskets) Process(ctx context.Context, userID, limit, relativeOffset int, offsetsLookup OffsetsLookup, since *time.Time, result *wdk.SyncChunk) (num int, err error) {
	offset := offsetsLookup[wdk.OutputBasketEntityName] + relativeOffset

	opts := []queryopts.QueryOptsUnion{
		queryopts.WithPage(queryopts.Paging{Limit: limit, Offset: offset}),
	}

	if since != nil {
		opts = append(opts, queryopts.WithSince(queryopts.Since{Time: *since}))
	}

	rows, err := c.repo.FindBasketsByUserID(ctx, userID, opts...)
	if err != nil {
		return 0, fmt.Errorf("fetch baskets by user id: %w", err)
	}

	result.OutputBaskets = append(result.OutputBaskets, rows...)

	return len(rows), nil
}
