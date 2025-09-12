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

// Transaction defines write and read access to transaction records.
type Transaction interface {
	Read() TransactionReader
	Create(ctx context.Context, tx *entity.Transaction) error
	Update(ctx context.Context, spec *entity.TransactionUpdateSpecification) error
}

// TransactionReadOperations provides basic query execution.
type TransactionReadOperations interface {
	Find(ctx context.Context) ([]*entity.Transaction, error)
	Count(ctx context.Context) (int64, error)
}

// TransactionReader enables fluent query building.
type TransactionReader interface {
	TransactionReadOperations

	ID(id uint) TransactionReadOperations
	UserID() NumericCondition[TransactionReader, int]
	Status() StringEnumCondition[TransactionReader, wdk.TxStatus]
	Reference() StringCondition[TransactionReader]
	IsOutgoing() BoolCondition[TransactionReader]
	Satoshis() NumericCondition[TransactionReader, int64]
	TxID() StringCondition[TransactionReader]
	DescriptionContains() StringCondition[TransactionReader]
	Labels() StringSetCondition[TransactionReader]
	Since(value time.Time, column entity.SinceField) TransactionReader
	Paged(limit, offset int, desc bool) TransactionReader
}

type transactionRepo interface {
	AddTransaction(ctx context.Context, tx *entity.Transaction) error
	UpdateTransaction(ctx context.Context, spec *entity.TransactionUpdateSpecification) error
	FindTransactions(ctx context.Context, spec *entity.TransactionReadSpecification, opts ...queryopts.Options) ([]*entity.Transaction, error)
	CountTransactions(ctx context.Context, spec *entity.TransactionReadSpecification, opts ...queryopts.Options) (int64, error)
}

type transaction struct {
	repo           transactionRepo
	spec           entity.TransactionReadSpecification
	pagingAndSince pagingAndSinceParams
}

// NewTransaction creates a new transaction CRUD instance.
func NewTransaction(repo transactionRepo) Transaction {
	return &transaction{
		repo: repo,
	}
}

func (t *transaction) Read() TransactionReader {
	return t
}

func (t *transaction) Create(ctx context.Context, tx *entity.Transaction) error {
	if tx == nil {
		return fmt.Errorf("transaction is nil")
	}
	err := t.repo.AddTransaction(ctx, tx)
	if err != nil {
		return fmt.Errorf("failed to create transaction: %w", err)
	}
	return nil
}

func (t *transaction) Update(ctx context.Context, spec *entity.TransactionUpdateSpecification) error {
	if spec == nil {
		return fmt.Errorf("update specification is nil")
	}
	err := t.repo.UpdateTransaction(ctx, spec)
	if err != nil {
		return fmt.Errorf("failed to update transaction: %w", err)
	}
	return nil
}

func (t *transaction) Find(ctx context.Context) ([]*entity.Transaction, error) {
	txs, err := t.repo.FindTransactions(ctx, &t.spec, t.pagingAndSince.QueryOpts()...)
	if err != nil {
		return nil, fmt.Errorf("failed to find transactions: %w", err)
	}
	return txs, nil
}

func (t *transaction) Count(ctx context.Context) (int64, error) {
	count, err := t.repo.CountTransactions(ctx, &t.spec, t.pagingAndSince.Since()...)
	if err != nil {
		return 0, fmt.Errorf("failed to count transactions: %w", err)
	}
	return count, nil
}

func (t *transaction) ID(id uint) TransactionReadOperations {
	t.spec.ID = to.Ptr(id)
	return t
}

func (t *transaction) UserID() NumericCondition[TransactionReader, int] {
	return &numericCondition[TransactionReader, int]{
		parent: t,
		conditionSetter: func(c *entity.Comparable[int]) {
			t.spec.UserID = c
		},
	}
}

func (t *transaction) Status() StringEnumCondition[TransactionReader, wdk.TxStatus] {
	return &stringEnumCondition[TransactionReader, wdk.TxStatus]{
		parent: t,
		conditionSetter: func(c *entity.Comparable[wdk.TxStatus]) {
			t.spec.Status = c
		},
	}
}

func (t *transaction) Reference() StringCondition[TransactionReader] {
	return &stringCondition[TransactionReader]{
		parent: t,
		conditionSetter: func(c *entity.Comparable[string]) {
			t.spec.Reference = c
		},
	}
}

func (t *transaction) IsOutgoing() BoolCondition[TransactionReader] {
	return &boolCondition[TransactionReader]{
		parent: t,
		conditionSetter: func(c *entity.Comparable[bool]) {
			t.spec.IsOutgoing = c
		},
	}
}

func (t *transaction) Satoshis() NumericCondition[TransactionReader, int64] {
	return &numericCondition[TransactionReader, int64]{
		parent: t,
		conditionSetter: func(c *entity.Comparable[int64]) {
			t.spec.Satoshis = c
		},
	}
}

func (t *transaction) TxID() StringCondition[TransactionReader] {
	return &stringCondition[TransactionReader]{
		parent: t,
		conditionSetter: func(c *entity.Comparable[string]) {
			t.spec.TxID = c
		},
	}
}

func (t *transaction) DescriptionContains() StringCondition[TransactionReader] {
	return &stringCondition[TransactionReader]{
		parent: t,
		conditionSetter: func(c *entity.Comparable[string]) {
			t.spec.DescriptionContains = c
		},
	}
}

func (t *transaction) Labels() StringSetCondition[TransactionReader] {
	return &stringSetCondition[TransactionReader]{
		parent: t,
		conditionSetter: func(c *entity.ComparableSet[string]) {
			t.spec.Labels = c
		},
	}
}

func (t *transaction) Since(value time.Time, column entity.SinceField) TransactionReader {
	t.pagingAndSince.since = &queryopts.Since{
		Time:  value,
		Field: to.IfThen(column == entity.SinceFieldCreatedAt, "created_at").ElseThen("updated_at"),
	}
	return t
}

func (t *transaction) Paged(limit, offset int, desc bool) TransactionReader {
	t.pagingAndSince.paging = &queryopts.Paging{
		Limit:  limit,
		Offset: offset,
		SortBy: "id",
		Sort:   to.IfThen(desc, "DESC").ElseThen("ASC"),
	}
	return t
}
