package crud

import (
	"context"
	"fmt"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/go-softwarelab/common/pkg/to"
)

// Commission provides query-building capabilities for retrieving and filtering commission records from a data source.
type Commission interface {
	Read() CommissionReader
	Create(ctx context.Context, commission *entity.Commission) error
	Update(ctx context.Context, spec *entity.CommissionUpdateSpecification) error
}

// CommissionReadOperations defines read operations for querying Commission entities from a data source.
type CommissionReadOperations interface {
	Find(ctx context.Context) ([]*entity.Commission, error)
	Count(ctx context.Context) (int64, error)
}

// CommissionReader provides a fluent interface for building commission queries with filtering and chaining conditions.
type CommissionReader interface {
	CommissionReadOperations

	ID(id uint) CommissionReadOperations
	Satoshis() NumericCondition[CommissionReader, uint64]
	TransactionID() NumericCondition[CommissionReader, uint]
	KeyOffset() StringCondition[CommissionReader]
	IsRedeemed(value bool) CommissionReader
	UserID(userID int) CommissionReader
	Since(value time.Time, column entity.SinceField) CommissionReader
	Paged(limit, offset int, desc bool) CommissionReader
}

type commissionRepo interface {
	FindCommissions(ctx context.Context, spec *entity.CommissionReadSpecification, opts ...queryopts.Options) ([]*entity.Commission, error)
	CountCommissions(ctx context.Context, spec *entity.CommissionReadSpecification, opts ...queryopts.Options) (int64, error)
	AddCommission(ctx context.Context, commission *entity.Commission) error
	UpdateCommission(ctx context.Context, spec *entity.CommissionUpdateSpecification) error
}

type commission struct {
	repo           commissionRepo
	spec           entity.CommissionReadSpecification
	pagingAndSince pagingAndSinceParams
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

func (c *commission) Create(ctx context.Context, commission *entity.Commission) error {
	if commission == nil {
		return fmt.Errorf("commission cannot be nil")
	}

	err := c.repo.AddCommission(ctx, commission)
	if err != nil {
		return fmt.Errorf("failed to create commission: %w", err)
	}
	return nil
}

func (c *commission) Update(ctx context.Context, spec *entity.CommissionUpdateSpecification) error {
	if spec == nil {
		return fmt.Errorf("update specification cannot be nil")
	}

	err := c.repo.UpdateCommission(ctx, spec)
	if err != nil {
		return fmt.Errorf("failed to update commission: %w", err)
	}
	return nil
}

func (c *commission) Find(ctx context.Context) ([]*entity.Commission, error) {
	commissions, err := c.repo.FindCommissions(ctx, &c.spec, c.pagingAndSince.QueryOpts()...)
	if err != nil {
		return nil, fmt.Errorf("failed to find commissions: %w", err)
	}
	return commissions, nil
}

func (c *commission) Count(ctx context.Context) (int64, error) {
	count, err := c.repo.CountCommissions(ctx, &c.spec, c.pagingAndSince.Since()...)
	if err != nil {
		return 0, fmt.Errorf("failed to count commissions: %w", err)
	}
	return count, nil
}

func (c *commission) ID(id uint) CommissionReadOperations {
	c.spec.ID = to.Ptr(id)
	return c
}

func (c *commission) IsRedeemed(value bool) CommissionReader {
	c.spec.IsRedeemed = to.Ptr(value)
	return c
}

func (c *commission) UserID(userID int) CommissionReader {
	c.spec.UserID = to.Ptr(userID)
	return c
}

func (c *commission) Satoshis() NumericCondition[CommissionReader, uint64] {
	return &numericCondition[CommissionReader, uint64]{
		parent: c,
		conditionSetter: func(spec *entity.Comparable[uint64]) {
			c.spec.Satoshis = spec
		},
	}
}

func (c *commission) TransactionID() NumericCondition[CommissionReader, uint] {
	return &numericCondition[CommissionReader, uint]{
		parent: c,
		conditionSetter: func(spec *entity.Comparable[uint]) {
			c.spec.TransactionID = spec
		},
	}
}

func (c *commission) KeyOffset() StringCondition[CommissionReader] {
	return &stringCondition[CommissionReader]{
		parent: c,
		conditionSetter: func(spec *entity.Comparable[string]) {
			c.spec.KeyOffset = spec
		},
	}
}

func (c *commission) Since(value time.Time, column entity.SinceField) CommissionReader {
	c.pagingAndSince.since = &queryopts.Since{
		Time:  value,
		Field: to.IfThen(column == entity.SinceFieldCreatedAt, "created_at").ElseThen("updated_at"),
	}
	return c
}

func (c *commission) Paged(limit, offset int, desc bool) CommissionReader {
	c.pagingAndSince.paging = &queryopts.Paging{
		Limit:  limit,
		Offset: offset,
		SortBy: "id",
		Sort:   to.IfThen(desc, "DESC").ElseThen("ASC"),
	}
	return c
}
