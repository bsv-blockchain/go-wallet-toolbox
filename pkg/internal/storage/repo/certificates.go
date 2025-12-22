package repo

import (
	"context"
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/genquery"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/scopes"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/tracing"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/slices"
	"go.opentelemetry.io/otel/attribute"
	"gorm.io/gen"
	"gorm.io/gorm"
)

type Certificates struct {
	db    *gorm.DB
	query *genquery.Query
}

func NewCertificates(db *gorm.DB, query *genquery.Query) *Certificates {
	return &Certificates{db: db, query: query}
}

func (c *Certificates) CreateCertificate(ctx context.Context, certificate *models.Certificate) (uint, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Certificates-CreateCertificate", attribute.String("SerialNumber", certificate.SerialNumber))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	err = c.db.WithContext(ctx).Create(certificate).Error
	if err != nil {
		return 0, fmt.Errorf("failed to create certificate model: %w", err)
	}
	return certificate.ID, nil
}

func (c *Certificates) DeleteCertificate(ctx context.Context, userID int, args wdk.RelinquishCertificateArgs) error {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Certificates-DeleteCertificate", attribute.String("SerialNumber", string(args.SerialNumber)), attribute.Int("UserID", userID))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	tx := c.db.WithContext(ctx).Delete(&models.Certificate{}, "type = ? AND serial_number = ? AND certifier = ? AND user_id = ?", args.Type, args.SerialNumber, args.Certifier, userID)
	if tx.RowsAffected == 0 {
		return fmt.Errorf("failed to delete certificate model: certificate not found")
	}
	if tx.Error != nil {
		return fmt.Errorf("failed to delete certificate model: %w", tx.Error)
	}

	return nil
}

func mapCertifierModelToEntity(model *models.Certificate) *entity.Certificate {
	return &entity.Certificate{
		ID:                 model.ID,
		CreatedAt:          model.CreatedAt,
		UpdatedAt:          model.UpdatedAt,
		Certifier:          model.Certifier,
		SerialNumber:       model.SerialNumber,
		UserID:             model.UserID,
		Type:               model.Type,
		Subject:            model.Subject,
		Verifier:           model.Verifier,
		RevocationOutpoint: model.RevocationOutpoint,
		Signature:          model.Signature,
		CertificateFields: slices.Map(model.CertificateFields, func(field *models.CertificateField) entity.CertificateField {
			return entity.CertificateField{
				CreatedAt:  field.CreatedAt,
				UpdatedAt:  field.UpdatedAt,
				FieldName:  field.FieldName,
				FieldValue: field.FieldValue,
				MasterKey:  field.MasterKey,
			}
		}),
	}
}

// FindCertifiers returns distinct certifiers for the given specification, with optional paging/since.
func (c *Certificates) FindCertifiers(ctx context.Context, spec *entity.CertificateReadSpecification, opts ...queryopts.Options) ([]*entity.Certificate, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Certificates-FindCertifiers")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	table := &c.query.Certificate

	certs, err := table.WithContext(ctx).
		Scopes(scopes.FromQueryOptsForGen(table, opts)...).
		Where(c.conditionsBySpec(spec)...).
		Preload(table.CertificateFields).
		Find()
	if err != nil {
		return nil, fmt.Errorf("failed to find certificates: %w", err)
	}

	return slices.Map(certs, mapCertifierModelToEntity), nil
}

// CountCertifiers returns the count of distinct certifiers matching the filters.
func (c *Certificates) CountCertifiers(ctx context.Context, spec *entity.CertificateReadSpecification, opts ...queryopts.Options) (int64, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Certificates-CountCertifiers")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	table := &c.query.Certificate

	count, err := table.WithContext(ctx).
		Scopes(scopes.FromQueryOptsForGen(table, opts)...).
		Where(c.conditionsBySpec(spec)...).
		Count()
	if err != nil {
		return 0, fmt.Errorf("failed to count users: %w", err)
	}

	return count, nil
}

func (c *Certificates) conditionsBySpec(spec *entity.CertificateReadSpecification) []gen.Condition {
	if spec == nil {
		return nil
	}

	table := &c.query.Certificate
	if spec.ID != nil {
		return []gen.Condition{table.ID.Eq(*spec.ID)}
	}

	var conditions []gen.Condition
	if spec.UserID != nil {
		conditions = append(conditions, cmpCondition(table.UserID, spec.UserID))
	}
	if spec.SerialNumber != nil {
		conditions = append(conditions, cmpCondition(table.SerialNumber, spec.SerialNumber))
	}
	if spec.Certifier != nil {
		conditions = append(conditions, cmpCondition(table.Certifier, spec.Certifier))
	}
	if spec.Type != nil {
		conditions = append(conditions, cmpCondition(table.Type, spec.Type))
	}
	if spec.Subject != nil {
		conditions = append(conditions, cmpCondition(table.Subject, spec.Subject))
	}
	if spec.Verifier != nil {
		conditions = append(conditions, cmpCondition(table.Verifier, spec.Verifier))
	}
	if spec.RevocationOutpoint != nil {
		conditions = append(conditions, cmpCondition(table.RevocationOutpoint, spec.RevocationOutpoint))
	}
	if spec.Signature != nil {
		conditions = append(conditions, cmpCondition(table.Signature, spec.Signature))
	}

	return conditions
}
