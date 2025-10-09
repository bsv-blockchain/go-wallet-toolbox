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

// Output provides CRUD operations for Output entities.
type Output interface {
	Read() OutputReader
	Create(ctx context.Context, output *entity.Output) error
	Update(ctx context.Context, spec *entity.OutputUpdateSpecification) error
}

// OutputReadOperations defines query execution methods.
type OutputReadOperations interface {
	Find(ctx context.Context) ([]*entity.Output, error)
	Count(ctx context.Context) (int64, error)
}

// OutputReader enables fluent query building for outputs.
type OutputReader interface {
	OutputReadOperations

	ID(id uint) OutputReadOperations
	UserID() NumericCondition[OutputReader, int]
	TransactionID() NumericCondition[OutputReader, uint]
	SpentBy() NumericCondition[OutputReader, uint]
	BasketName() StringCondition[OutputReader]
	Spendable() BoolCondition[OutputReader]
	Change() BoolCondition[OutputReader]
	TxStatus() StringEnumCondition[OutputReader, wdk.TxStatus]
	Satoshis() NumericCondition[OutputReader, int64]
	TxID() StringCondition[OutputReader]
	Vout() NumericCondition[OutputReader, uint32]
	Tags() StringSetCondition[OutputReader]

	Since(value time.Time, column entity.SinceField) OutputReader
	Paged(limit, offset int, desc bool) OutputReader
}

// outputRepo defines storage-level operations.
type outputRepo interface {
	AddOutput(ctx context.Context, output *entity.Output) error
	UpdateOutput(ctx context.Context, spec *entity.OutputUpdateSpecification) error
	FindOutputs(ctx context.Context, spec *entity.OutputReadSpecification, opts ...queryopts.Options) ([]*entity.Output, error)
	CountOutputs(ctx context.Context, spec *entity.OutputReadSpecification, opts ...queryopts.Options) (int64, error)
}

type output struct {
	repo           outputRepo
	spec           entity.OutputReadSpecification
	pagingAndSince pagingAndSinceParams
}

// NewOutput creates a new Output CRUD instance.
func NewOutput(repo outputRepo) Output {
	return &output{repo: repo}
}

func (o *output) Read() OutputReader {
	return o
}

func (o *output) Create(ctx context.Context, out *entity.Output) error {
	if out == nil {
		return fmt.Errorf("output is nil")
	}
	if err := o.repo.AddOutput(ctx, out); err != nil {
		return fmt.Errorf("failed to create output: %w", err)
	}
	return nil
}

func (o *output) Update(ctx context.Context, spec *entity.OutputUpdateSpecification) error {
	if spec == nil {
		return fmt.Errorf("update specification is nil")
	}
	if err := o.repo.UpdateOutput(ctx, spec); err != nil {
		return fmt.Errorf("failed to update output: %w", err)
	}
	return nil
}

func (o *output) Find(ctx context.Context) ([]*entity.Output, error) {
	results, err := o.repo.FindOutputs(ctx, &o.spec, o.pagingAndSince.QueryOpts()...)
	if err != nil {
		return nil, fmt.Errorf("failed to find outputs: %w", err)
	}
	return results, nil
}

func (o *output) Count(ctx context.Context) (int64, error) {
	count, err := o.repo.CountOutputs(ctx, &o.spec, o.pagingAndSince.Since()...)
	if err != nil {
		return 0, fmt.Errorf("failed to count outputs: %w", err)
	}
	return count, nil
}

func (o *output) ID(id uint) OutputReadOperations {
	o.spec.ID = to.Ptr(id)
	return o
}

func (o *output) UserID() NumericCondition[OutputReader, int] {
	return &numericCondition[OutputReader, int]{
		parent: o,
		conditionSetter: func(c *entity.Comparable[int]) {
			o.spec.UserID = c
		},
	}
}

func (o *output) TransactionID() NumericCondition[OutputReader, uint] {
	return &numericCondition[OutputReader, uint]{
		parent: o,
		conditionSetter: func(c *entity.Comparable[uint]) {
			o.spec.TransactionID = c
		},
	}
}

func (o *output) SpentBy() NumericCondition[OutputReader, uint] {
	return &numericCondition[OutputReader, uint]{
		parent: o,
		conditionSetter: func(c *entity.Comparable[uint]) {
			o.spec.SpentBy = c
		},
	}
}

func (o *output) BasketName() StringCondition[OutputReader] {
	return &stringCondition[OutputReader]{
		parent: o,
		conditionSetter: func(c *entity.Comparable[string]) {
			o.spec.BasketName = c
		},
	}
}

func (o *output) Spendable() BoolCondition[OutputReader] {
	return &boolCondition[OutputReader]{
		parent: o,
		conditionSetter: func(c *entity.Comparable[bool]) {
			o.spec.Spendable = c
		},
	}
}

func (o *output) Change() BoolCondition[OutputReader] {
	return &boolCondition[OutputReader]{
		parent: o,
		conditionSetter: func(c *entity.Comparable[bool]) {
			o.spec.Change = c
		},
	}
}

func (o *output) TxStatus() StringEnumCondition[OutputReader, wdk.TxStatus] {
	return &stringEnumCondition[OutputReader, wdk.TxStatus]{
		parent: o,
		conditionSetter: func(c *entity.Comparable[wdk.TxStatus]) {
			o.spec.TxStatus = c
		},
	}
}

func (o *output) Satoshis() NumericCondition[OutputReader, int64] {
	return &numericCondition[OutputReader, int64]{
		parent: o,
		conditionSetter: func(c *entity.Comparable[int64]) {
			o.spec.Satoshis = c
		},
	}
}

func (o *output) TxID() StringCondition[OutputReader] {
	return &stringCondition[OutputReader]{
		parent: o,
		conditionSetter: func(c *entity.Comparable[string]) {
			o.spec.TxID = c
		},
	}
}

func (o *output) Vout() NumericCondition[OutputReader, uint32] {
	return &numericCondition[OutputReader, uint32]{
		parent: o,
		conditionSetter: func(c *entity.Comparable[uint32]) {
			o.spec.Vout = c
		},
	}
}

func (o *output) Tags() StringSetCondition[OutputReader] {
	return &stringSetCondition[OutputReader]{
		parent: o,
		conditionSetter: func(c *entity.ComparableSet[string]) {
			o.spec.Tags = c
		},
	}
}

func (o *output) Since(value time.Time, column entity.SinceField) OutputReader {
	o.pagingAndSince.since = &queryopts.Since{
		Time:  value,
		Field: to.IfThen(column == entity.SinceFieldCreatedAt, "created_at").ElseThen("updated_at"),
	}
	return o
}

func (o *output) Paged(limit, offset int, desc bool) OutputReader {
	o.pagingAndSince.paging = &queryopts.Paging{
		Limit:  limit,
		Offset: offset,
		SortBy: "id",
		Sort:   to.IfThen(desc, "DESC").ElseThen("ASC"),
	}
	return o
}
