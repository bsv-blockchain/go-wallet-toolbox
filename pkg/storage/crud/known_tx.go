package crud

import (
	"context"
	"fmt"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/to"
)

// KnownTx provides query-building capabilities for retrieving and filtering known transaction records from a data source.
type KnownTx interface {
	Read() KnownTxReader
}

// KnownTxReadOperations defines read operations for querying KnownTx entities from a data source.
type KnownTxReadOperations interface {
	Find(ctx context.Context) ([]*entity.KnownTx, error)
	Count(ctx context.Context) (int64, error)

	IncludeHistoryNotes() KnownTxReader
}

// KnownTxReader provides a fluent interface for building known transaction queries with filtering and chaining conditions.
type KnownTxReader interface {
	KnownTxReadOperations

	TxID(txID string) KnownTxReadOperations
	TxIDs(txIDs ...string) KnownTxReadOperations
	Attempts() NumericCondition[KnownTxReader, uint64]
	Status() StringEnumCondition[KnownTxReader, wdk.ProvenTxReqStatus]
	Notified() BoolCondition[KnownTxReader]
	BlockHeight() NumericCondition[KnownTxReader, uint32]
	MerkleRoot() StringCondition[KnownTxReader]
	BlockHash() StringCondition[KnownTxReader]

	Since(value time.Time, column entity.SinceField) KnownTxReader
	Paged(limit, offset int, desc bool) KnownTxReader
}

type knownTxRepo interface {
	FindKnownTxs(ctx context.Context, spec *entity.KnownTxReadSpecification, opts ...queryopts.Options) ([]*entity.KnownTx, error)
	CountKnownTxs(ctx context.Context, spec *entity.KnownTxReadSpecification, opts ...queryopts.Options) (int64, error)
}

// NewKnownTx creates and returns a new KnownTx instance using the provided knownTxRepo implementation.
// The returned KnownTx can be used to build queries for known tx records with various filters and options.
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

func (k *knownTx) TxIDs(txIDs ...string) KnownTxReadOperations {
	k.spec.TxIDs = txIDs
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
		SortBy: "tx_id",
		Sort:   to.IfThen(desc, "DESC").ElseThen("ASC"),
	}
	return k
}

func (k *knownTx) IncludeHistoryNotes() KnownTxReader {
	k.spec.IncludeHistoryNotes = true
	return k
}

func (k *knownTx) Attempts() NumericCondition[KnownTxReader, uint64] {
	return &numericCondition[KnownTxReader, uint64]{
		parent: k,
		conditionSetter: func(spec *entity.Comparable[uint64]) {
			k.spec.Attempts = spec
		},
	}
}

func (k *knownTx) Status() StringEnumCondition[KnownTxReader, wdk.ProvenTxReqStatus] {
	return &stringEnumCondition[KnownTxReader, wdk.ProvenTxReqStatus]{
		parent: k,
		conditionSetter: func(spec *entity.Comparable[wdk.ProvenTxReqStatus]) {
			k.spec.Status = spec
		},
	}
}

func (k *knownTx) Notified() BoolCondition[KnownTxReader] {
	return &boolCondition[KnownTxReader]{
		parent: k,
		conditionSetter: func(spec *entity.Comparable[bool]) {
			k.spec.Notified = spec
		},
	}
}

func (k *knownTx) BlockHeight() NumericCondition[KnownTxReader, uint32] {
	return &numericCondition[KnownTxReader, uint32]{
		parent: k,
		conditionSetter: func(spec *entity.Comparable[uint32]) {
			k.spec.BlockHeight = spec
		},
	}
}

func (k *knownTx) MerkleRoot() StringCondition[KnownTxReader] {
	return &stringCondition[KnownTxReader]{
		parent: k,
		conditionSetter: func(spec *entity.Comparable[string]) {
			k.spec.MerkleRoot = spec
		},
	}
}

func (k *knownTx) BlockHash() StringCondition[KnownTxReader] {
	return &stringCondition[KnownTxReader]{
		parent: k,
		conditionSetter: func(spec *entity.Comparable[string]) {
			k.spec.BlockHash = spec
		},
	}
}
