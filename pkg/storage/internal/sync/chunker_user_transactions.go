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
	maxUserTransactionsPageSize = 4000
)

type chunkerUserTransactions struct {
	repo Repository
}

func newChunkerUserTransactions(repo Repository) *chunkerUserTransactions {
	return &chunkerUserTransactions{
		repo: repo,
	}
}

func (c *chunkerUserTransactions) Name() string {
	return "user_transactions"
}

func (c *chunkerUserTransactions) MaxPageSize() uint64 {
	return maxUserTransactionsPageSize
}

func (c *chunkerUserTransactions) IsApplicable(requestedEntities OffsetsLookup) bool {
	_, ok := requestedEntities[wdk.TransactionEntityName]
	return ok
}

func (c *chunkerUserTransactions) FirstPage(offsetsLookup OffsetsLookup) *queryopts.Paging {
	offset := offsetsLookup[wdk.TransactionEntityName]
	return &queryopts.Paging{
		Offset: must.ConvertToIntFromUnsigned(offset),
	}
}

func (c *chunkerUserTransactions) Process(ctx context.Context, userID int, page *queryopts.Paging, since *time.Time, result *wdk.SyncChunk) (num uint64, err error) {
	opts := chunkerQueryOptions(page, since)

	transactions, err := c.repo.FindTransactionsForSync(ctx, userID, opts...)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch proven transactions by user id: %w", err)
	}

	result.Transactions = append(result.Transactions, transactions...)

	return must.ConvertToUInt64(len(transactions)), nil
}
