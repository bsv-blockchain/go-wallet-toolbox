package sync

import (
	"context"
	"fmt"
	"time"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/must"
)

const (
	maxTransactionsPageSize = 1000
)

type chunkerTransactions struct {
	repo Repository
}

func newChunkerTransactions(repo Repository) *chunkerTransactions {
	return &chunkerTransactions{
		repo: repo,
	}
}

func (c *chunkerTransactions) Name() string {
	return "transactions"
}

func (c *chunkerTransactions) MaxPageSize() uint64 {
	return maxTransactionsPageSize
}

func (c *chunkerTransactions) IsApplicable(requestedEntities OffsetsLookup) bool {
	return c.requestedProvenTxReq(requestedEntities) || c.requestedProvenTx(requestedEntities)
}

func (c *chunkerTransactions) requestedProvenTx(requestedEntities OffsetsLookup) bool {
	_, ok := requestedEntities[wdk.ProvenTxEntityName]
	return ok
}

func (c *chunkerTransactions) requestedProvenTxReq(requestedEntities OffsetsLookup) bool {
	_, ok := requestedEntities[wdk.ProvenTxReqEntityName]
	return ok
}

func (c *chunkerTransactions) FirstPage(offsetsLookup OffsetsLookup) *queryopts.Paging {
	offset := offsetsLookup[wdk.ProvenTxReqEntityName] + offsetsLookup[wdk.ProvenTxEntityName]
	return &queryopts.Paging{
		Offset: must.ConvertToIntFromUnsigned(offset),
	}
}

func (c *chunkerTransactions) Process(ctx context.Context, userID int, page *queryopts.Paging, since *time.Time, result *wdk.SyncChunk) (num uint64, err error) {
	opts := []queryopts.Options{
		queryopts.WithPage(*page),
	}

	if since != nil {
		opts = append(opts, queryopts.WithSince(queryopts.Since{Time: *since}))
	}

	reqs, mined, err := c.repo.FindKnownTxsForSync(ctx, userID, opts...)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch proven transactions by user id: %w", err)
	}

	result.ProvenTxReqs = append(result.ProvenTxReqs, reqs...)
	result.ProvenTxs = append(result.ProvenTxs, mined...)

	return must.ConvertToUInt64(len(reqs) + len(mined)), nil
}
