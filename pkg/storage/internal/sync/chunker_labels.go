package sync

import (
	"context"
	"fmt"
	"time"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/must"
)

type chunkerLabels struct {
	repo Repository
}

func newChunkerLabels(repo Repository) *chunkerLabels {
	return &chunkerLabels{
		repo: repo,
	}
}

func (c *chunkerLabels) Name() string {
	return "labels"
}

func (c *chunkerLabels) MaxPageSize() uint64 {
	return maximumAvailablePageSize
}

func (c *chunkerLabels) IsApplicable(requestedEntities OffsetsLookup) bool {
	_, ok := requestedEntities[wdk.TxLabelEntityName]
	return ok
}

func (c *chunkerLabels) FirstPage(offsetsLookup OffsetsLookup) *queryopts.Paging {
	offset := offsetsLookup[wdk.TxLabelEntityName]
	return &queryopts.Paging{
		Offset: must.ConvertToIntFromUnsigned(offset),
	}
}

func (c *chunkerLabels) Process(ctx context.Context, userID int, page *queryopts.Paging, since *time.Time, result *wdk.SyncChunk) (num uint64, err error) {
	opts := chunkerQueryOptions(page, since)

	rows, err := c.repo.FindLabelsForSync(ctx, userID, opts...)
	if err != nil {
		return 0, fmt.Errorf("fetch baskets by user id: %w", err)
	}

	result.TxLabels = append(result.TxLabels, rows...)

	return must.ConvertToUInt64(len(rows)), nil
}
