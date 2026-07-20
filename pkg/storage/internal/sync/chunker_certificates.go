package sync

import (
	"context"
	"fmt"
	"time"

	"github.com/go-softwarelab/common/pkg/must"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type chunkerCertificates struct {
	repo Repository
}

func newChunkerCertificates(repo Repository) *chunkerCertificates {
	return &chunkerCertificates{
		repo: repo,
	}
}

func (c *chunkerCertificates) Name() string {
	return "certificates"
}

func (c *chunkerCertificates) MaxPageSize() uint64 {
	return maximumAvailablePageSize
}

func (c *chunkerCertificates) IsApplicable(requestedEntities OffsetsLookup) bool {
	_, ok := requestedEntities[wdk.CertificateEntityName]
	return ok
}

func (c *chunkerCertificates) FirstPage(offsetsLookup OffsetsLookup) *queryopts.Paging {
	offset := offsetsLookup[wdk.CertificateEntityName]
	return &queryopts.Paging{
		Offset: must.ConvertToIntFromUnsigned(offset),
	}
}

func (c *chunkerCertificates) Process(ctx context.Context, userID int, page *queryopts.Paging, since *time.Time, result *wdk.SyncChunk) (num uint64, err error) {
	opts := chunkerQueryOptions(page, since)

	rows, err := c.repo.FindCertificatesForSync(ctx, userID, opts...)
	if err != nil {
		return 0, fmt.Errorf("fetch certificates by user id: %w", err)
	}

	result.Certificates = append(result.Certificates, rows...)

	return must.ConvertToUInt64(len(rows)), nil
}

type chunkerCertificateFields struct {
	repo Repository
}

func newChunkerCertificateFields(repo Repository) *chunkerCertificateFields {
	return &chunkerCertificateFields{
		repo: repo,
	}
}

func (c *chunkerCertificateFields) Name() string {
	return "certificate_fields"
}

func (c *chunkerCertificateFields) MaxPageSize() uint64 {
	return maximumAvailablePageSize
}

func (c *chunkerCertificateFields) IsApplicable(requestedEntities OffsetsLookup) bool {
	_, ok := requestedEntities[wdk.CertificateFieldEntityName]
	return ok
}

func (c *chunkerCertificateFields) FirstPage(offsetsLookup OffsetsLookup) *queryopts.Paging {
	offset := offsetsLookup[wdk.CertificateFieldEntityName]
	return &queryopts.Paging{
		Offset: must.ConvertToIntFromUnsigned(offset),
	}
}

func (c *chunkerCertificateFields) Process(ctx context.Context, userID int, page *queryopts.Paging, since *time.Time, result *wdk.SyncChunk) (num uint64, err error) {
	opts := chunkerQueryOptions(page, since)

	rows, err := c.repo.FindCertificateFieldsForSync(ctx, userID, opts...)
	if err != nil {
		return 0, fmt.Errorf("fetch certificate fields by user id: %w", err)
	}

	result.CertificateFields = append(result.CertificateFields, rows...)

	return must.ConvertToUInt64(len(rows)), nil
}
