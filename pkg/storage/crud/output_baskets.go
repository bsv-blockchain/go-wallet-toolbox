package crud

import (
	"context"
	"fmt"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/go-softwarelab/common/pkg/to"
)

// OutputBasket is an interface for managing OutputBasket entities.
type OutputBasket interface {
	Read() OutputBasketReader
	Create(ctx context.Context, basket *entity.OutputBasket) error
	Update(ctx context.Context, spec *entity.OutputBasketUpdateSpecification) error
}

// OutputBasketReadOperations is an interface for reading OutputBasket entities.
type OutputBasketReadOperations interface {
	Find(ctx context.Context) ([]*entity.OutputBasket, error)
	Count(ctx context.Context) (int64, error)
}

// OutputBasketReader is an interface for reading OutputBasket entities.
type OutputBasketReader interface {
	OutputBasketReadOperations
	UserID() NumericCondition[OutputBasketReader, int]
	Name() StringCondition[OutputBasketReader]
	NumberOfDesiredUTXOs() NumericCondition[OutputBasketReader, int64]
	MinimumDesiredUTXOValue() NumericCondition[OutputBasketReader, uint64]
	Since(value time.Time, column entity.SinceField) OutputBasketReader
	Paged(limit, offset int, desc bool) OutputBasketReader
}

// outputBasketRepo is an interface for managing OutputBasket entities in the database.
type outputBasketRepo interface {
	AddOutputBasket(ctx context.Context, basket *entity.OutputBasket) error
	UpdateOutputBasket(ctx context.Context, spec *entity.OutputBasketUpdateSpecification) error
	FindOutputBaskets(ctx context.Context, spec *entity.OutputBasketReadSpecification, opts ...queryopts.Options) ([]*entity.OutputBasket, error)
	CountOutputBaskets(ctx context.Context, spec *entity.OutputBasketReadSpecification, opts ...queryopts.Options) (int64, error)
}

type outputBasket struct {
	repo           outputBasketRepo
	spec           entity.OutputBasketReadSpecification
	pagingAndSince pagingAndSinceParams
}

// NewOutputBasket creates a new OutputBasket instance.
func NewOutputBasket(repo outputBasketRepo) OutputBasket {
	return &outputBasket{repo: repo}
}

// Read returns an OutputBasketReader for reading OutputBasket entities.
func (o *outputBasket) Read() OutputBasketReader {
	return o
}

// Create adds a new OutputBasket entity to the database.
func (o *outputBasket) Create(ctx context.Context, basket *entity.OutputBasket) error {
	if basket == nil {
		return fmt.Errorf("output basket cannot be nil")
	}
	if err := o.repo.AddOutputBasket(ctx, basket); err != nil {
		return fmt.Errorf("failed to add output basket: %w", err)
	}
	return nil
}

// Update modifies an existing OutputBasket entity in the database.
func (o *outputBasket) Update(ctx context.Context, spec *entity.OutputBasketUpdateSpecification) error {
	if spec == nil {
		return fmt.Errorf("update specification cannot be nil")
	}
	if err := o.repo.UpdateOutputBasket(ctx, spec); err != nil {
		return fmt.Errorf("failed to update output basket: %w", err)
	}
	return nil
}

// Find returns a list of OutputBasket entities from the database.
func (o *outputBasket) Find(ctx context.Context) ([]*entity.OutputBasket, error) {
	baskets, err := o.repo.FindOutputBaskets(ctx, &o.spec, o.pagingAndSince.QueryOpts()...)
	if err != nil {
		return nil, fmt.Errorf("failed to find output baskets: %w", err)
	}
	return baskets, nil
}

// Count returns the count of OutputBasket entities from the database.
func (o *outputBasket) Count(ctx context.Context) (int64, error) {
	count, err := o.repo.CountOutputBaskets(ctx, &o.spec, o.pagingAndSince.Since()...)
	if err != nil {
		return 0, fmt.Errorf("failed to count output baskets: %w", err)
	}
	return count, nil
}

func (o *outputBasket) UserID() NumericCondition[OutputBasketReader, int] {
	return &numericCondition[OutputBasketReader, int]{
		parent: o,
		conditionSetter: func(spec *entity.Comparable[int]) {
			o.spec.UserID = spec
		},
	}
}

func (o *outputBasket) Name() StringCondition[OutputBasketReader] {
	return &stringCondition[OutputBasketReader]{
		parent: o,
		conditionSetter: func(spec *entity.Comparable[string]) {
			o.spec.Name = spec
		},
	}
}

func (o *outputBasket) NumberOfDesiredUTXOs() NumericCondition[OutputBasketReader, int64] {
	return &numericCondition[OutputBasketReader, int64]{
		parent: o,
		conditionSetter: func(spec *entity.Comparable[int64]) {
			o.spec.NumberOfDesiredUTXOs = spec
		},
	}
}

func (o *outputBasket) MinimumDesiredUTXOValue() NumericCondition[OutputBasketReader, uint64] {
	return &numericCondition[OutputBasketReader, uint64]{
		parent: o,
		conditionSetter: func(spec *entity.Comparable[uint64]) {
			o.spec.MinimumDesiredUTXOValue = spec
		},
	}
}

func (o *outputBasket) Since(value time.Time, column entity.SinceField) OutputBasketReader {
	o.pagingAndSince.since = &queryopts.Since{
		Time:  value,
		Field: to.IfThen(column == entity.SinceFieldCreatedAt, "created_at").ElseThen("updated_at"),
	}
	return o
}

func (o *outputBasket) Paged(limit, offset int, desc bool) OutputBasketReader {
	o.pagingAndSince.paging = &queryopts.Paging{
		Limit:  limit,
		Offset: offset,
		SortBy: "created_at",
		Sort:   to.IfThen(desc, "DESC").ElseThen("ASC"),
	}
	return o
}
