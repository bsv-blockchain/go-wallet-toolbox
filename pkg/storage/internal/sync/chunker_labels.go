package sync

import (
	"context"
	"fmt"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
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
		return 0, fmt.Errorf("fetch labels by user id: %w", err)
	}

	result.TxLabels = append(result.TxLabels, rows...)

	return must.ConvertToUInt64(len(rows)), nil
}

type chunkerLabelsMap struct {
	repo Repository
}

func newChunkerLabelsMap(repo Repository) *chunkerLabelsMap {
	return &chunkerLabelsMap{
		repo: repo,
	}
}

func (c *chunkerLabelsMap) Name() string {
	return "labels_map"
}

func (c *chunkerLabelsMap) MaxPageSize() uint64 {
	return maximumAvailablePageSize
}

func (c *chunkerLabelsMap) IsApplicable(requestedEntities OffsetsLookup) bool {
	_, ok := requestedEntities[wdk.TxLabelMapEntityName]
	return ok
}

func (c *chunkerLabelsMap) FirstPage(offsetsLookup OffsetsLookup) *queryopts.Paging {
	offset := offsetsLookup[wdk.TxLabelMapEntityName]
	return &queryopts.Paging{
		Offset: must.ConvertToIntFromUnsigned(offset),
	}
}

func (c *chunkerLabelsMap) Process(ctx context.Context, userID int, page *queryopts.Paging, since *time.Time, result *wdk.SyncChunk) (num uint64, err error) {
	opts := chunkerQueryOptions(page, since)

	rows, err := c.repo.FindLabelsMapForSync(ctx, userID, opts...)
	if err != nil {
		return 0, fmt.Errorf("fetch labels map by user id: %w", err)
	}

	result.TxLabelMaps = append(result.TxLabelMaps, rows...)

	return must.ConvertToUInt64(len(rows)), nil
}
