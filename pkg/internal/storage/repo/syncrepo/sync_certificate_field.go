package syncrepo

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-softwarelab/common/pkg/slices"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/genquery"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/scopes"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

// SyncCertificateField handles certificate field read/upsert for cross-storage sync.
type SyncCertificateField struct {
	db    *gorm.DB
	query *genquery.Query
}

// NewSyncCertificateField constructs a SyncCertificateField repository.
func NewSyncCertificateField(db *gorm.DB, query *genquery.Query) *SyncCertificateField {
	return &SyncCertificateField{db: db, query: query}
}

func (s *SyncCertificateField) tableName() string {
	// CertificateField is not a top-level genquery table; derive from the model schema.
	stmt := &gorm.Statement{DB: s.db}
	if err := stmt.Parse(&models.CertificateField{}); err != nil {
		return "bsv_certificate_fields"
	}
	return stmt.Schema.Table
}

// FindCertificateFieldsForSync returns user certificate fields for getSyncChunk.
func (s *SyncCertificateField) FindCertificateFieldsForSync(ctx context.Context, userID int, opts ...queryopts.Options) ([]*wdk.TableCertificateField, error) {
	queryopts.ModifyOptions(opts, func(options *queryopts.Options) {
		if options.Since != nil && options.Since.TableName == "" {
			options.Since.TableName = s.tableName()
		}
	})
	filters := append(scopes.FromQueryOpts(opts), scopes.UserID(userID))

	var resultModels []*models.CertificateField
	err := s.db.WithContext(ctx).
		Model(&models.CertificateField{}).
		Scopes(filters...).
		// Deterministic tiebreak after Paginate's created_at DESC.
		Scopes(func(db *gorm.DB) *gorm.DB {
			return db.Order(clause.OrderByColumn{Column: clause.Column{Table: s.tableName(), Name: "field_name"}}).
				Order(clause.OrderByColumn{Column: clause.Column{Table: s.tableName(), Name: "certificate_id"}})
		}).
		Find(&resultModels).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find certificate fields for sync: %w", err)
	}

	return slices.Map(resultModels, s.mapModelToTableCertificateField), nil
}

// UpsertCertificateFieldForSync inserts or updates a certificate field for processSyncChunk.
// Natural key: certificate_id (writer-side) + user_id + field_name.
// BRC-40: only apply UPDATE when incoming.updated_at is strictly newer.
// Note: certificateField has no ID map (see wdk.SyncMapEntity).
func (s *SyncCertificateField) UpsertCertificateFieldForSync(ctx context.Context, e *entity.CertificateField) (isNew bool, err error) {
	if e == nil {
		return false, fmt.Errorf("certificate field entity is nil")
	}

	model := models.CertificateField{
		CreatedAt:     e.CreatedAt,
		UpdatedAt:     e.UpdatedAt,
		FieldName:     e.FieldName,
		FieldValue:    e.FieldValue,
		MasterKey:     e.MasterKey,
		UserID:        e.UserID,
		CertificateID: e.CertificateID,
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing models.CertificateField
		existsErr := tx.Model(&models.CertificateField{}).
			Select("field_name, certificate_id, updated_at").
			Where("user_id = ? AND certificate_id = ? AND field_name = ?", e.UserID, e.CertificateID, e.FieldName).
			First(&existing).Error

		if existsErr == nil {
			if !model.UpdatedAt.After(existing.UpdatedAt) {
				return nil
			}

			updateTx := tx.Model(&models.CertificateField{}).
				Where("user_id = ? AND certificate_id = ? AND field_name = ? AND updated_at < ?",
					e.UserID, e.CertificateID, e.FieldName, model.UpdatedAt).
				Updates(map[string]any{
					"field_value": model.FieldValue,
					"master_key":  model.MasterKey,
					"updated_at":  model.UpdatedAt,
				})
			if updateTx.Error != nil {
				return fmt.Errorf("failed to update certificate field: %w", updateTx.Error)
			}
			return nil
		}

		if !errors.Is(existsErr, gorm.ErrRecordNotFound) {
			return fmt.Errorf("failed to lookup existing certificate field: %w", existsErr)
		}

		// Skip BeforeCreate OnConflict DoNothing so create is a real insert for sync.
		if err = tx.Session(&gorm.Session{SkipHooks: true}).Create(&model).Error; err != nil {
			return fmt.Errorf("failed to create certificate field: %w", err)
		}

		isNew = true
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("transaction failed: %w", err)
	}

	return isNew, nil
}

func (s *SyncCertificateField) mapModelToTableCertificateField(model *models.CertificateField) *wdk.TableCertificateField {
	return &wdk.TableCertificateField{
		CreatedAt:     model.CreatedAt,
		UpdatedAt:     model.UpdatedAt,
		UserID:        model.UserID,
		CertificateID: model.CertificateID,
		FieldName:     model.FieldName,
		FieldValue:    model.FieldValue,
		MasterKey:     primitives.Base64String(model.MasterKey),
	}
}
