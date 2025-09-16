package crud

import (
	"context"
	"fmt"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/go-softwarelab/common/pkg/to"
)

// TxNote provides query access for TxNote entities.
type TxNote interface {
	Read() TxNoteReader
}

// TxNoteReadOperations defines basic read operations for TxNote entities.
type TxNoteReadOperations interface {
	Find(ctx context.Context) ([]*entity.TxNotes, error)
	Count(ctx context.Context) (int64, error)
}

// TxNoteReader defines a fluent query builder interface for TxNote queries.
type TxNoteReader interface {
	TxNoteReadOperations

	TxID(txID string) TxNoteReadOperations
	UserID() NumericCondition[TxNoteReader, int]
	What() StringCondition[TxNoteReader]
	CreatedAt() TimeCondition[TxNoteReader]

	Since(value time.Time, column entity.SinceField) TxNoteReader
	Paged(limit, offset int, desc bool) TxNoteReader
}

// txNoteRepo defines the repository interface used by the CRUD layer.
type txNoteRepo interface {
	FindTxNotes(ctx context.Context, spec *entity.TxNoteReadSpecification, opts ...queryopts.Options) ([]*entity.TxNotes, error)
	CountTxNotes(ctx context.Context, spec *entity.TxNoteReadSpecification, opts ...queryopts.Options) (int64, error)
}

// txNote is the concrete implementation of TxNoteReader.
type txNote struct {
	repo           txNoteRepo
	spec           entity.TxNoteReadSpecification
	pagingAndSince pagingAndSinceParams
}

// NewTxNote creates a new TxNote instance.
func NewTxNote(repo txNoteRepo) TxNote {
	return &txNote{repo: repo}
}

func (t *txNote) Read() TxNoteReader {
	return t
}

func (t *txNote) Find(ctx context.Context) ([]*entity.TxNotes, error) {
	notes, err := t.repo.FindTxNotes(ctx, &t.spec, t.pagingAndSince.QueryOpts()...)
	if err != nil {
		return nil, fmt.Errorf("failed to find tx notes: %w", err)
	}
	return notes, nil
}

func (t *txNote) Count(ctx context.Context) (int64, error) {
	count, err := t.repo.CountTxNotes(ctx, &t.spec, t.pagingAndSince.Since()...)
	if err != nil {
		return 0, fmt.Errorf("failed to count tx notes: %w", err)
	}
	return count, nil
}

func (t *txNote) TxID(txID string) TxNoteReadOperations {
	t.spec.TxID = to.Ptr(txID)
	return t
}

func (t *txNote) UserID() NumericCondition[TxNoteReader, int] {
	return &numericCondition[TxNoteReader, int]{
		parent: t,
		conditionSetter: func(cond *entity.Comparable[int]) {
			t.spec.UserID = cond
		},
	}
}

func (t *txNote) What() StringCondition[TxNoteReader] {
	return &stringCondition[TxNoteReader]{
		parent: t,
		conditionSetter: func(cond *entity.Comparable[string]) {
			t.spec.What = cond
		},
	}
}

func (t *txNote) CreatedAt() TimeCondition[TxNoteReader] {
	return &timeCondition[TxNoteReader]{
		parent: t,
		conditionSetter: func(cond *entity.Comparable[time.Time]) {
			t.spec.CreatedAt = cond
		},
	}
}

func (t *txNote) Paged(limit, offset int, desc bool) TxNoteReader {
	t.pagingAndSince.paging = &queryopts.Paging{
		Limit:  limit,
		Offset: offset,
		SortBy: "id",
		Sort:   to.IfThen(desc, "DESC").ElseThen("ASC"),
	}
	return t
}

func (t *txNote) Since(value time.Time, column entity.SinceField) TxNoteReader {
	t.pagingAndSince.since = &queryopts.Since{
		Time:  value,
		Field: to.IfThen(column == entity.SinceFieldCreatedAt, "created_at").ElseThen("updated_at"),
	}
	return t
}
