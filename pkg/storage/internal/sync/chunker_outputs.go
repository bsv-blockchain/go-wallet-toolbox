package sync

import (
	"context"
	"fmt"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/must"
)

const (
	maxOutputsPageSize = 4000
)

type chunkerOutputs struct {
	repo Repository
}

func newChunkerOutputs(repo Repository) *chunkerOutputs {
	return &chunkerOutputs{
		repo: repo,
	}
}

func (c *chunkerOutputs) Name() string {
	return "outputs"
}

func (c *chunkerOutputs) MaxPageSize() uint64 {
	return maxOutputsPageSize
}

func (c *chunkerOutputs) IsApplicable(requestedEntities OffsetsLookup) bool {
	_, ok := requestedEntities[wdk.OutputEntityName]
	return ok
}

func (c *chunkerOutputs) FirstPage(offsetsLookup OffsetsLookup) *queryopts.Paging {
	offset := offsetsLookup[wdk.OutputEntityName]
	return &queryopts.Paging{
		Offset: must.ConvertToIntFromUnsigned(offset),
	}
}

func (c *chunkerOutputs) Process(ctx context.Context, userID int, page *queryopts.Paging, since *time.Time, result *wdk.SyncChunk) (num uint64, err error) {
	opts := chunkerQueryOptions(page, since)

	outputs, err := c.repo.FindOutputsForSync(ctx, userID, opts...)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch proven transactions by user id: %w", err)
	}

	result.Outputs = append(result.Outputs, outputs...)

	return must.ConvertToUInt64(len(outputs)), nil
}
