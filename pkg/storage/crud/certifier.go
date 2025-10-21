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
	Read() CertificateReader
}

// CertifierReadOperations defines read operations for querying Certifier entities.
type CertifierReadOperations interface {
	Find(ctx context.Context) ([]*entity.Certificate, error)
	Count(ctx context.Context) (int64, error)
}

// CertificateReader provides a fluent interface for building certificate queries.
type CertificateReader interface {
	CertifierReadOperations

	ID(id uint) CertifierReadOperations
	SerialNumber() StringCondition[CertificateReader]
	Subject() StringCondition[CertificateReader]
	Verifier() StringCondition[CertificateReader]
	RevocationOutpoint() StringCondition[CertificateReader]
	Signature() StringCondition[CertificateReader]
	UserID() NumericCondition[CertificateReader, int]
	Certifier() StringCondition[CertificateReader]
	Type() StringCondition[CertificateReader]

	Since(value time.Time, column entity.SinceField) CertificateReader
	Paged(limit, offset int, desc bool) CertificateReader
}

type certificateRepo interface {
	FindCertifiers(ctx context.Context, spec *entity.CertificateReadSpecification, opts ...queryopts.Options) ([]*entity.Certificate, error)
	CountCertifiers(ctx context.Context, spec *entity.CertificateReadSpecification, opts ...queryopts.Options) (int64, error)
}

type certificate struct {
	repo           certificateRepo
	spec           entity.CertificateReadSpecification
	pagingAndSince pagingAndSinceParams
}

// NewCertificate creates a new Certifier query builder instance.
func NewCertificate(repo certificateRepo) Certifier {
	return &certificate{repo: repo}
}

func (c *certificate) Read() CertificateReader { return c }

func (c *certificate) Find(ctx context.Context) ([]*entity.Certificate, error) {
	rows, err := c.repo.FindCertifiers(ctx, &c.spec, c.pagingAndSince.QueryOpts()...)
	if err != nil {
		return nil, fmt.Errorf("failed to find certifiers: %w", err)
	}
	return rows, nil
}

func (c *certificate) Count(ctx context.Context) (int64, error) {
	count, err := c.repo.CountCertifiers(ctx, &c.spec, c.pagingAndSince.Since()...)
	if err != nil {
		return 0, fmt.Errorf("failed to count certifiers: %w", err)
	}
	return count, nil
}

func (c *certificate) ID(id uint) CertifierReadOperations {
	c.spec.ID = to.Ptr(id)
	return c
}

func (c *certificate) UserID() NumericCondition[CertificateReader, int] {
	return &numericCondition[CertificateReader, int]{
		parent: c,
		conditionSetter: func(cond *entity.Comparable[int]) {
			c.spec.UserID = cond
		},
	}
}

func (c *certificate) Certifier() StringCondition[CertificateReader] {
	return &stringCondition[CertificateReader]{
		parent: c,
		conditionSetter: func(cond *entity.Comparable[string]) {
			c.spec.Certifier = cond
		},
	}
}

func (c *certificate) SerialNumber() StringCondition[CertificateReader] {
	return &stringCondition[CertificateReader]{
		parent: c,
		conditionSetter: func(cond *entity.Comparable[string]) {
			c.spec.SerialNumber = cond
		},
	}
}

func (c *certificate) Type() StringCondition[CertificateReader] {
	return &stringCondition[CertificateReader]{
		parent: c,
		conditionSetter: func(cond *entity.Comparable[string]) {
			c.spec.Type = cond
		},
	}
}

func (c *certificate) Subject() StringCondition[CertificateReader] {
	return &stringCondition[CertificateReader]{
		parent: c,
		conditionSetter: func(cond *entity.Comparable[string]) {
			c.spec.Subject = cond
		},
	}
}

func (c *certificate) Verifier() StringCondition[CertificateReader] {
	return &stringCondition[CertificateReader]{
		parent: c,
		conditionSetter: func(cond *entity.Comparable[string]) {
			c.spec.Verifier = cond
		},
	}
}

func (c *certificate) RevocationOutpoint() StringCondition[CertificateReader] {
	return &stringCondition[CertificateReader]{
		parent: c,
		conditionSetter: func(cond *entity.Comparable[string]) {
			c.spec.RevocationOutpoint = cond
		},
	}
}

func (c *certificate) Signature() StringCondition[CertificateReader] {
	return &stringCondition[CertificateReader]{
		parent: c,
		conditionSetter: func(cond *entity.Comparable[string]) {
			c.spec.Signature = cond
		},
	}
}

func (c *certificate) Since(value time.Time, column entity.SinceField) CertificateReader {
	c.pagingAndSince.since = &queryopts.Since{
		Time:  value,
		Field: to.IfThen(column == entity.SinceFieldCreatedAt, "created_at").ElseThen("updated_at"),
	}
	return c
}

func (c *certificate) Paged(limit, offset int, desc bool) CertificateReader {
	c.pagingAndSince.paging = &queryopts.Paging{
		Limit:  limit,
		Offset: offset,
		SortBy: "id",
		Sort:   to.IfThen(desc, "DESC").ElseThen("ASC"),
	}
	return c
}
