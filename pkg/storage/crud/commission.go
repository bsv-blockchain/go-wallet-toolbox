package crud

import (
	"context"
	"fmt"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/go-softwarelab/common/pkg/types"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/queryopts"
)

type Commission interface {
	Read() CommissionReader
}

type CommissionReadOperations interface {
	Find(ctx context.Context, opts ...queryopts.Options) ([]*entity.Commission, error)
}

type NumericCondition[T comparable] interface {
	Equals(value T) CommissionReader
	GreaterThan(value T) CommissionReader
	LessThan(value T) CommissionReader
	GreaterThanOrEqual(value T) CommissionReader
	LessThanOrEqual(value T) CommissionReader
}

type CommissionReader interface {
	CommissionReadOperations

	ID(id uint) CommissionReadOperations
	Satoshis() NumericCondition[uint64]
	IsRedeemed(value bool) CommissionReader
}

type commissionRepo interface {
	FindCommissions(ctx context.Context, spec *entity.CommissionSpecification, opts ...queryopts.Options) ([]*entity.Commission, error)
}

type commission struct {
	repo commissionRepo
	spec entity.CommissionSpecification
}

// NewCommission creates and returns a new Commission instance using the provided commissionRepo implementation.
// The returned Commission can be used to build queries for commission records with various filters and options.
func NewCommission(repo commissionRepo) Commission {
	return &commission{
		repo: repo,
	}
}

func (c *commission) Read() CommissionReader {
	return c
}

func (c *commission) Find(ctx context.Context, opts ...queryopts.Options) ([]*entity.Commission, error) {
	commissions, err := c.repo.FindCommissions(ctx, &c.spec, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to find commissions: %w", err)
	}
	return commissions, nil
}

func (c *commission) ID(id uint) CommissionReadOperations {
	c.spec.ID = to.Ptr(id)
	return c
}

func (c *commission) IsRedeemed(value bool) CommissionReader {
	c.spec.IsRedeemed = to.Ptr(value)
	return c
}

func (c *commission) Satoshis() NumericCondition[uint64] {
	return &numericCondition[uint64]{
		parent: c,
		conditionSetter: func(spec *entity.ComparableNumber[uint64]) {
			c.spec.Satoshis = spec
		},
	}
}

type numericCondition[T types.Number] struct {
	parent          *commission
	conditionSetter func(spec *entity.ComparableNumber[T])
}

func (c *numericCondition[T]) Equals(value T) CommissionReader {
	c.conditionSetter(&entity.ComparableNumber[T]{
		Value: value,
		Cmp:   entity.Equal,
	})

	return c.parent
}

func (c *numericCondition[T]) GreaterThan(value T) CommissionReader {
	c.conditionSetter(&entity.ComparableNumber[T]{
		Value: value,
		Cmp:   entity.GreaterThan,
	})

	return c.parent
}

func (c *numericCondition[T]) LessThan(value T) CommissionReader {
	c.conditionSetter(&entity.ComparableNumber[T]{
		Value: value,
		Cmp:   entity.LessThan,
	})

	return c.parent
}

func (c *numericCondition[T]) GreaterThanOrEqual(value T) CommissionReader {
	c.conditionSetter(&entity.ComparableNumber[T]{
		Value: value,
		Cmp:   entity.GreaterThanOrEqual,
	})

	return c.parent
}

func (c *numericCondition[T]) LessThanOrEqual(value T) CommissionReader {
	c.conditionSetter(&entity.ComparableNumber[T]{
		Value: value,
		Cmp:   entity.LessThanOrEqual,
	})

	return c.parent
}
