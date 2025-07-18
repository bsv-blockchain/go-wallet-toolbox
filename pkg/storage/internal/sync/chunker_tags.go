package sync

import (
	"context"
	"fmt"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/must"
)

type chunkerTags struct {
	repo Repository
}

func newChunkerTags(repo Repository) *chunkerTags {
	return &chunkerTags{
		repo: repo,
	}
}

func (c *chunkerTags) Name() string {
	return "tags"
}

func (c *chunkerTags) MaxPageSize() uint64 {
	return maximumAvailablePageSize
}

func (c *chunkerTags) IsApplicable(requestedEntities OffsetsLookup) bool {
	_, ok := requestedEntities[wdk.OutputTagEntityName]
	return ok
}

func (c *chunkerTags) FirstPage(offsetsLookup OffsetsLookup) *queryopts.Paging {
	offset := offsetsLookup[wdk.OutputTagEntityName]
	return &queryopts.Paging{
		Offset: must.ConvertToIntFromUnsigned(offset),
	}
}

func (c *chunkerTags) Process(ctx context.Context, userID int, page *queryopts.Paging, since *time.Time, result *wdk.SyncChunk) (num uint64, err error) {
	opts := chunkerQueryOptions(page, since)

	rows, err := c.repo.FindTagsForSync(ctx, userID, opts...)
	if err != nil {
		return 0, fmt.Errorf("fetch tags by user id: %w", err)
	}

	result.OutputTags = append(result.OutputTags, rows...)

	return must.ConvertToUInt64(len(rows)), nil
}

type chunkerTagsMap struct {
	repo Repository
}

func newChunkerTagsMap(repo Repository) *chunkerTagsMap {
	return &chunkerTagsMap{
		repo: repo,
	}
}

func (c *chunkerTagsMap) Name() string {
	return "tags_map"
}

func (c *chunkerTagsMap) MaxPageSize() uint64 {
	return maximumAvailablePageSize
}

func (c *chunkerTagsMap) IsApplicable(requestedEntities OffsetsLookup) bool {
	_, ok := requestedEntities[wdk.OutputTagMapEntityName]
	return ok
}

func (c *chunkerTagsMap) FirstPage(offsetsLookup OffsetsLookup) *queryopts.Paging {
	offset := offsetsLookup[wdk.OutputTagMapEntityName]
	return &queryopts.Paging{
		Offset: must.ConvertToIntFromUnsigned(offset),
	}
}

func (c *chunkerTagsMap) Process(ctx context.Context, userID int, page *queryopts.Paging, since *time.Time, result *wdk.SyncChunk) (num uint64, err error) {
	opts := chunkerQueryOptions(page, since)

	rows, err := c.repo.FindTagsMapForSync(ctx, userID, opts...)
	if err != nil {
		return 0, fmt.Errorf("fetch tags map by user id: %w", err)
	}

	result.OutputTagMaps = append(result.OutputTagMaps, rows...)

	return must.ConvertToUInt64(len(rows)), nil
}
