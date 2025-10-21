package crud

import (
	"context"
	"fmt"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/go-softwarelab/common/pkg/to"
)

// Certifier provides query-building capabilities for retrieving distinct certifiers.
type Certifier interface {
	Read() CertifierReader
}

// CertifierReadOperations defines read operations for querying Certifier entities.
type CertifierReadOperations interface {
	Find(ctx context.Context) ([]*entity.Certifier, error)
	Count(ctx context.Context) (int64, error)
}

// CertifierReader provides a fluent interface for building certifier queries.
type CertifierReader interface {
	CertifierReadOperations

	UserID() NumericCondition[CertifierReader, int]
	Certifier() StringCondition[CertifierReader]
	Type() StringCondition[CertifierReader]

	Since(value time.Time, column entity.SinceField) CertifierReader
	Paged(limit, offset int, desc bool) CertifierReader
}

type certifierRepo interface {
	FindCertifiers(ctx context.Context, spec *entity.CertifierReadSpecification, opts ...queryopts.Options) ([]*entity.Certifier, error)
	CountCertifiers(ctx context.Context, spec *entity.CertifierReadSpecification, opts ...queryopts.Options) (int64, error)
}

type certifier struct {
	repo           certifierRepo
	spec           entity.CertifierReadSpecification
	pagingAndSince pagingAndSinceParams
}

// NewCertifier creates a new Certifier query builder instance.
func NewCertifier(repo certifierRepo) Certifier {
	return &certifier{repo: repo}
}

func (c *certifier) Read() CertifierReader { return c }

func (c *certifier) Find(ctx context.Context) ([]*entity.Certifier, error) {
	rows, err := c.repo.FindCertifiers(ctx, &c.spec, c.pagingAndSince.QueryOpts()...)
	if err != nil {
		return nil, fmt.Errorf("failed to find certifiers: %w", err)
	}
	return rows, nil
}

func (c *certifier) Count(ctx context.Context) (int64, error) {
	count, err := c.repo.CountCertifiers(ctx, &c.spec, c.pagingAndSince.Since()...)
	if err != nil {
		return 0, fmt.Errorf("failed to count certifiers: %w", err)
	}
	return count, nil
}

func (c *certifier) UserID() NumericCondition[CertifierReader, int] {
	return &numericCondition[CertifierReader, int]{
		parent: c,
		conditionSetter: func(cond *entity.Comparable[int]) {
			c.spec.UserID = cond
		},
	}
}

func (c *certifier) Certifier() StringCondition[CertifierReader] {
	return &stringCondition[CertifierReader]{
		parent: c,
		conditionSetter: func(cond *entity.Comparable[string]) {
			c.spec.Certifier = cond
		},
	}
}

func (c *certifier) Type() StringCondition[CertifierReader] {
	return &stringCondition[CertifierReader]{
		parent: c,
		conditionSetter: func(cond *entity.Comparable[string]) {
			c.spec.Type = cond
		},
	}
}

func (c *certifier) Subject() StringCondition[CertifierReader] {
	return &stringCondition[CertifierReader]{
		parent: c,
		conditionSetter: func(cond *entity.Comparable[string]) {
			c.spec.Subject = cond
		},
	}
}

func (c *certifier) Verifier() StringCondition[CertifierReader] {
	return &stringCondition[CertifierReader]{
		parent: c,
		conditionSetter: func(cond *entity.Comparable[string]) {
			c.spec.Verifier = cond
		},
	}
}

func (c *certifier) RevocationOutpoint() StringCondition[CertifierReader] {
	return &stringCondition[CertifierReader]{
		parent: c,
		conditionSetter: func(cond *entity.Comparable[string]) {
			c.spec.RevocationOutpoint = cond
		},
	}
}

func (c *certifier) Signature() StringCondition[CertifierReader] {
	return &stringCondition[CertifierReader]{
		parent: c,
		conditionSetter: func(cond *entity.Comparable[string]) {
			c.spec.Signature = cond
		},
	}
}

func (c *certifier) Since(value time.Time, column entity.SinceField) CertifierReader {
	c.pagingAndSince.since = &queryopts.Since{
		Time:  value,
		Field: to.IfThen(column == entity.SinceFieldCreatedAt, "created_at").ElseThen("updated_at"),
	}
	return c
}

func (c *certifier) Paged(limit, offset int, desc bool) CertifierReader {
	c.pagingAndSince.paging = &queryopts.Paging{
		Limit:  limit,
		Offset: offset,
		SortBy: "certifier",
		Sort:   to.IfThen(desc, "DESC").ElseThen("ASC"),
	}
	return c
}
