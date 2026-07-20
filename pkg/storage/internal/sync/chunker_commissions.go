package sync

import (
	"context"
	"fmt"
	"time"

	"github.com/go-softwarelab/common/pkg/must"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type chunkerCommissions struct {
	repo Repository
}

func newChunkerCommissions(repo Repository) *chunkerCommissions {
	return &chunkerCommissions{
		repo: repo,
	}
}

func (c *chunkerCommissions) Name() string {
	return "commissions"
}

func (c *chunkerCommissions) MaxPageSize() uint64 {
	return maximumAvailablePageSize
}

func (c *chunkerCommissions) IsApplicable(requestedEntities OffsetsLookup) bool {
	_, ok := requestedEntities[wdk.CommissionEntityName]
	return ok
}

func (c *chunkerCommissions) FirstPage(offsetsLookup OffsetsLookup) *queryopts.Paging {
	offset := offsetsLookup[wdk.CommissionEntityName]
	return &queryopts.Paging{
		Offset: must.ConvertToIntFromUnsigned(offset),
	}
}

func (c *chunkerCommissions) Process(ctx context.Context, userID int, page *queryopts.Paging, since *time.Time, result *wdk.SyncChunk) (num uint64, err error) {
	opts := chunkerQueryOptions(page, since)

	rows, err := c.repo.FindCommissionsForSync(ctx, userID, opts...)
	if err != nil {
		return 0, fmt.Errorf("fetch commissions by user id: %w", err)
	}

	result.Commissions = append(result.Commissions, rows...)

	return must.ConvertToUInt64(len(rows)), nil
}
