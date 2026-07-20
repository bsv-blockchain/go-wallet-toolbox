package syncrepo

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-softwarelab/common/pkg/slices"
	"github.com/go-softwarelab/common/pkg/to"
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

// SyncCertificate handles certificate read/upsert for cross-storage sync.
type SyncCertificate struct {
	db    *gorm.DB
	query *genquery.Query
}

// NewSyncCertificate constructs a SyncCertificate repository.
func NewSyncCertificate(db *gorm.DB, query *genquery.Query) *SyncCertificate {
	return &SyncCertificate{db: db, query: query}
}

func (s *SyncCertificate) tableName() string {
	return s.query.Certificate.TableName()
}

// FindCertificatesForSync returns user certificates (including soft-deleted) for getSyncChunk.
func (s *SyncCertificate) FindCertificatesForSync(ctx context.Context, userID int, opts ...queryopts.Options) ([]*wdk.TableCertificate, error) {
	queryopts.ModifyOptions(opts, func(options *queryopts.Options) {
		if options.Since != nil && options.Since.TableName == "" {
			options.Since.TableName = s.tableName()
		}
	})
	filters := append(scopes.FromQueryOpts(opts), scopes.UserID(userID))

	var resultModels []*models.Certificate
	err := s.db.WithContext(ctx).
		Model(&models.Certificate{}).
		// Include soft-deleted certificates so isDeleted is preserved across sync.
		Unscoped().
		Scopes(filters...).
		// Deterministic tiebreak after Paginate's created_at DESC (see FindOutputsForSync).
		Scopes(func(db *gorm.DB) *gorm.DB {
			return db.Order(clause.OrderByColumn{Column: clause.Column{Table: s.tableName(), Name: "id"}})
		}).
		Find(&resultModels).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find certificates for sync: %w", err)
	}

	return slices.Map(resultModels, s.mapModelToTableCertificate), nil
}

// UpsertCertificateForSync inserts or updates a certificate for processSyncChunk.
// Natural key (reader-side mergeFind): serial_number + certifier + user_id.
// BRC-40: only apply UPDATE when incoming.updated_at is strictly newer.
func (s *SyncCertificate) UpsertCertificateForSync(ctx context.Context, e *entity.Certificate) (isNew bool, certificateID uint, err error) {
	if e == nil {
		return false, 0, fmt.Errorf("certificate entity is nil")
	}

	model := models.Certificate{
		Model: gorm.Model{
			CreatedAt: e.CreatedAt,
			UpdatedAt: e.UpdatedAt,
		},
		Type:               e.Type,
		SerialNumber:       e.SerialNumber,
		Certifier:          e.Certifier,
		Subject:            e.Subject,
		Verifier:           e.Verifier,
		RevocationOutpoint: e.RevocationOutpoint,
		Signature:          e.Signature,
		UserID:             e.UserID,
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing models.Certificate
		// Unscoped so we can restore a previously soft-deleted certificate.
		existsErr := tx.Unscoped().
			Model(&models.Certificate{}).
			Select("id, updated_at, deleted_at").
			Where("user_id = ? AND serial_number = ? AND certifier = ?", e.UserID, e.SerialNumber, e.Certifier).
			First(&existing).Error

		if existsErr == nil {
			if !model.UpdatedAt.After(existing.UpdatedAt) {
				certificateID = existing.ID
				return nil
			}

			updateMap := map[string]any{
				"type":                model.Type,
				"subject":             model.Subject,
				"serial_number":       model.SerialNumber,
				"certifier":           model.Certifier,
				"verifier":            model.Verifier,
				"revocation_outpoint": model.RevocationOutpoint,
				"signature":           model.Signature,
				"updated_at":          model.UpdatedAt,
			}
			if e.IsDeleted {
				updateMap["deleted_at"] = model.UpdatedAt
			} else {
				// Expr("NULL") forces a real NULL write; map nil is skipped by GORM.
				updateMap["deleted_at"] = gorm.Expr("NULL")
			}

			updateTx := tx.Unscoped().
				Model(&models.Certificate{}).
				Where("id = ? AND updated_at < ?", existing.ID, model.UpdatedAt).
				Updates(updateMap)
			if updateTx.Error != nil {
				return fmt.Errorf("failed to update certificate: %w", updateTx.Error)
			}

			certificateID = existing.ID
			return nil
		}

		if !errors.Is(existsErr, gorm.ErrRecordNotFound) {
			return fmt.Errorf("failed to lookup existing certificate: %w", existsErr)
		}

		if err = tx.Create(&model).Error; err != nil {
			return fmt.Errorf("failed to create certificate: %w", err)
		}
		if model.ID == 0 {
			return fmt.Errorf("certificate ID is zero after creation")
		}

		if e.IsDeleted {
			if err = tx.Delete(&models.Certificate{}, model.ID).Error; err != nil {
				return fmt.Errorf("failed to soft-delete newly created certificate: %w", err)
			}
		}

		isNew = true
		certificateID = model.ID
		return nil
	})
	if err != nil {
		return false, 0, fmt.Errorf("transaction failed: %w", err)
	}

	return isNew, certificateID, nil
}

func (s *SyncCertificate) mapModelToTableCertificate(model *models.Certificate) *wdk.TableCertificate {
	var verifier *primitives.PubKeyHex
	if model.Verifier != "" {
		verifier = to.Ptr(primitives.PubKeyHex(model.Verifier))
	}

	return &wdk.TableCertificate{
		CreatedAt:          model.CreatedAt,
		UpdatedAt:          model.UpdatedAt,
		CertificateID:      model.ID,
		UserID:             model.UserID,
		Type:               primitives.Base64String(model.Type),
		SerialNumber:       primitives.Base64String(model.SerialNumber),
		Certifier:          primitives.PubKeyHex(model.Certifier),
		Subject:            primitives.PubKeyHex(model.Subject),
		Verifier:           verifier,
		RevocationOutpoint: primitives.OutpointString(model.RevocationOutpoint),
		Signature:          primitives.HexString(model.Signature),
		IsDeleted:          model.DeletedAt.Valid,
	}
}
