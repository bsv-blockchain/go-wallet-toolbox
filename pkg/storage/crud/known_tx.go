package crud

import (
	"context"
	"fmt"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/go-softwarelab/common/pkg/to"
	"time"
)

type KnownTx interface {
	Read() KnownTxReader
}

type KnownTxReadOperations interface {
	Find(ctx context.Context) ([]*entity.KnownTx, error)
	Count(ctx context.Context) (int64, error)
}

type KnownTxReader interface {
	KnownTxReadOperations

	TxID(txID string) KnownTxReadOperations
	Since(value time.Time, column entity.SinceField) KnownTxReader
	Paged(limit, offset int, desc bool) KnownTxReader
}

type knownTxRepo interface {
	FindKnownTxs(ctx context.Context, spec *entity.KnownTxReadSpecification, opts ...queryopts.Options) ([]*entity.KnownTx, error)
	CountKnownTxs(ctx context.Context, spec *entity.KnownTxReadSpecification, opts ...queryopts.Options) (int64, error)
}

func NewKnownTx(repo knownTxRepo) KnownTx {
	return &knownTx{
		repo: repo,
	}
}

type knownTx struct {
	repo           knownTxRepo
	spec           entity.KnownTxReadSpecification
	pagingAndSince pagingAndSinceParams
}

func (k *knownTx) Read() KnownTxReader {
	return k
}

func (k *knownTx) Find(ctx context.Context) ([]*entity.KnownTx, error) {
	knownTxs, err := k.repo.FindKnownTxs(ctx, &k.spec, k.pagingAndSince.QueryOpts()...)
	if err != nil {
		return nil, fmt.Errorf("failed to find known transactions: %w", err)
	}
	return knownTxs, nil
}

func (k *knownTx) Count(ctx context.Context) (int64, error) {
	count, err := k.repo.CountKnownTxs(ctx, &k.spec, k.pagingAndSince.Since()...)
	if err != nil {
		return 0, fmt.Errorf("failed to count known transactions: %w", err)
	}
	return count, nil
}

func (k *knownTx) TxID(txID string) KnownTxReadOperations {
	k.spec.TxID = to.Ptr(txID)
	return k
}

func (k *knownTx) Since(value time.Time, column entity.SinceField) KnownTxReader {
	k.pagingAndSince.since = &queryopts.Since{
		Time:  value,
		Field: to.IfThen(column == entity.SinceFieldCreatedAt, "created_at").ElseThen("updated_at"),
	}
	return k
}

func (k *knownTx) Paged(limit, offset int, desc bool) KnownTxReader {
	k.pagingAndSince.paging = &queryopts.Paging{
		Limit:  limit,
		Offset: offset,
		SortBy: "id",
		Sort:   to.IfThen(desc, "DESC").ElseThen("ASC"),
	}
	return k
}
