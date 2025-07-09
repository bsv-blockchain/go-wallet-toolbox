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
	maxTransactionsPageSize = 1000
)

type chunkerKnownTxs struct {
	repo Repository
}

func newChunkerKnownTxs(repo Repository) *chunkerKnownTxs {
	return &chunkerKnownTxs{
		repo: repo,
	}
}

func (c *chunkerKnownTxs) Name() string {
	return "known_transactions"
}

func (c *chunkerKnownTxs) MaxPageSize() uint64 {
	return maxTransactionsPageSize
}

func (c *chunkerKnownTxs) IsApplicable(requestedEntities OffsetsLookup) bool {
	return c.requestedProvenTxReq(requestedEntities) || c.requestedProvenTx(requestedEntities)
}

func (c *chunkerKnownTxs) requestedProvenTx(requestedEntities OffsetsLookup) bool {
	_, ok := requestedEntities[wdk.ProvenTxEntityName]
	return ok
}

func (c *chunkerKnownTxs) requestedProvenTxReq(requestedEntities OffsetsLookup) bool {
	_, ok := requestedEntities[wdk.ProvenTxReqEntityName]
	return ok
}

func (c *chunkerKnownTxs) FirstPage(offsetsLookup OffsetsLookup) *queryopts.Paging {
	offset := offsetsLookup[wdk.ProvenTxReqEntityName] + offsetsLookup[wdk.ProvenTxEntityName]
	return &queryopts.Paging{
		Offset: must.ConvertToIntFromUnsigned(offset),
	}
}

func (c *chunkerKnownTxs) Process(ctx context.Context, userID int, page *queryopts.Paging, since *time.Time, result *wdk.SyncChunk) (num uint64, err error) {
	opts := chunkerQueryOptions(page, since)

	reqs, mined, err := c.repo.FindKnownTxsForSync(ctx, userID, opts...)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch known transactions by user id: %w", err)
	}

	result.ProvenTxReqs = append(result.ProvenTxReqs, reqs...)
	result.ProvenTxs = append(result.ProvenTxs, mined...)

	return must.ConvertToUInt64(len(reqs) + len(mined)), nil
}
