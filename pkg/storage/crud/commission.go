package crud

import (
	"context"
	"fmt"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/go-softwarelab/common/pkg/to"
)

// Commission provides query-building capabilities for retrieving and filtering commission records from a data source.
type Commission interface {
	Read() CommissionReader
}

// CommissionReadOperations defines read operations for querying Commission entities from a data source.
type CommissionReadOperations interface {
	Find(ctx context.Context, opts ...queryopts.Options) ([]*entity.Commission, error)
}

// CommissionReader provides a fluent interface for building commission queries with filtering and chaining conditions.
type CommissionReader interface {
	CommissionReadOperations

	ID(id uint) CommissionReadOperations
	Satoshis() NumericCondition[CommissionReader, uint64]
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

func (c *commission) Satoshis() NumericCondition[CommissionReader, uint64] {
	return &numericCondition[CommissionReader, uint64]{
		parent: c,
		conditionSetter: func(spec *entity.ComparableNumber[uint64]) {
			c.spec.Satoshis = spec
		},
	}
}
