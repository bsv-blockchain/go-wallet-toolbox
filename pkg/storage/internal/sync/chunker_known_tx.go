package sync

import (
	"context"
	"fmt"
	"time"

	"github.com/go-softwarelab/common/pkg/must"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

const (
	maxTransactionsPageSize = 1000
)

// chunkerProvenTxReqs and chunkerProvenTxs are independent chunkers (rather than one combined
// chunker) because ProvenTxReq and ProvenTx are separately-offset/counted sync entities: sharing
// a single page/offset between the two queries (as a prior version of this code did, by summing
// the two entities' offsets into one combined offset) breaks pagination, since each query would
// independently consume the full page limit, letting a single Process() call return up to
// 2x the requested item budget.

type chunkerProvenTxReqs struct {
	repo Repository
}

func newChunkerProvenTxReqs(repo Repository) *chunkerProvenTxReqs {
	return &chunkerProvenTxReqs{
		repo: repo,
	}
}

func (c *chunkerProvenTxReqs) Name() string {
	return "proven_tx_reqs"
}

func (c *chunkerProvenTxReqs) MaxPageSize() uint64 {
	return maxTransactionsPageSize
}

func (c *chunkerProvenTxReqs) IsApplicable(requestedEntities OffsetsLookup) bool {
	_, ok := requestedEntities[wdk.ProvenTxReqEntityName]
	return ok
}

func (c *chunkerProvenTxReqs) FirstPage(offsetsLookup OffsetsLookup) *queryopts.Paging {
	return &queryopts.Paging{
		Offset: must.ConvertToIntFromUnsigned(offsetsLookup[wdk.ProvenTxReqEntityName]),
	}
}

func (c *chunkerProvenTxReqs) Process(ctx context.Context, userID int, page *queryopts.Paging, since *time.Time, result *wdk.SyncChunk) (num uint64, err error) {
	opts := chunkerQueryOptions(page, since)

	reqs, err := c.repo.FindProvenTxReqsForSync(ctx, userID, opts...)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch proven tx reqs by user id: %w", err)
	}

	result.ProvenTxReqs = append(result.ProvenTxReqs, reqs...)

	return must.ConvertToUInt64(len(reqs)), nil
}

type chunkerProvenTxs struct {
	repo Repository
}

func newChunkerProvenTxs(repo Repository) *chunkerProvenTxs {
	return &chunkerProvenTxs{
		repo: repo,
	}
}

func (c *chunkerProvenTxs) Name() string {
	return "proven_txs"
}

func (c *chunkerProvenTxs) MaxPageSize() uint64 {
	return maxTransactionsPageSize
}

func (c *chunkerProvenTxs) IsApplicable(requestedEntities OffsetsLookup) bool {
	_, ok := requestedEntities[wdk.ProvenTxEntityName]
	return ok
}

func (c *chunkerProvenTxs) FirstPage(offsetsLookup OffsetsLookup) *queryopts.Paging {
	return &queryopts.Paging{
		Offset: must.ConvertToIntFromUnsigned(offsetsLookup[wdk.ProvenTxEntityName]),
	}
}

func (c *chunkerProvenTxs) Process(ctx context.Context, userID int, page *queryopts.Paging, since *time.Time, result *wdk.SyncChunk) (num uint64, err error) {
	opts := chunkerQueryOptions(page, since)

	mined, err := c.repo.FindProvenTxsForSync(ctx, userID, opts...)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch proven txs by user id: %w", err)
	}

	result.ProvenTxs = append(result.ProvenTxs, mined...)

	return must.ConvertToUInt64(len(mined)), nil
}
