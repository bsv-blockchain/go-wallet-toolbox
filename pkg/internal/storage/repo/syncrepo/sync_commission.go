package syncrepo

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-softwarelab/common/pkg/must"
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

// SyncCommission handles commission read/upsert for cross-storage sync.
type SyncCommission struct {
	db    *gorm.DB
	query *genquery.Query
}

// NewSyncCommission constructs a SyncCommission repository.
func NewSyncCommission(db *gorm.DB, query *genquery.Query) *SyncCommission {
	return &SyncCommission{db: db, query: query}
}

func (s *SyncCommission) tableName() string {
	return s.query.Commission.TableName()
}

// FindCommissionsForSync returns user commissions for getSyncChunk.
func (s *SyncCommission) FindCommissionsForSync(ctx context.Context, userID int, opts ...queryopts.Options) ([]*wdk.TableCommission, error) {
	queryopts.ModifyOptions(opts, func(options *queryopts.Options) {
		if options.Since != nil && options.Since.TableName == "" {
			options.Since.TableName = s.tableName()
		}
	})
	filters := append(scopes.FromQueryOpts(opts), scopes.UserID(userID))

	var resultModels []*models.Commission
	err := s.db.WithContext(ctx).
		Model(&models.Commission{}).
		Scopes(filters...).
		// Deterministic tiebreak after Paginate's created_at DESC (see FindOutputsForSync).
		Scopes(func(db *gorm.DB) *gorm.DB {
			return db.Order(clause.OrderByColumn{Column: clause.Column{Table: s.tableName(), Name: "id"}})
		}).
		Find(&resultModels).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find commissions for sync: %w", err)
	}

	return slices.Map(resultModels, s.mapModelToTableCommission), nil
}

// UpsertCommissionForSync inserts or updates a commission for processSyncChunk.
// Natural key (reader-side mergeFind): transaction_id (writer-side) + user_id.
// BRC-40: only apply UPDATE when incoming.updated_at is strictly newer.
// On update, only is_redeemed (and timestamps) change per TS EntityCommission.mergeExisting.
func (s *SyncCommission) UpsertCommissionForSync(ctx context.Context, e *entity.Commission) (isNew bool, commissionID uint, err error) {
	if e == nil {
		return false, 0, fmt.Errorf("commission entity is nil")
	}

	model := commissionModelFromEntity(e)

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		created, id, upsertErr := s.upsertCommissionModel(tx, e, model)
		if upsertErr != nil {
			return upsertErr
		}
		isNew = created
		commissionID = id
		return nil
	})
	if err != nil {
		return false, 0, fmt.Errorf("transaction failed: %w", err)
	}

	return isNew, commissionID, nil
}

func commissionModelFromEntity(e *entity.Commission) models.Commission {
	return models.Commission{
		Model: gorm.Model{
			CreatedAt: e.CreatedAt,
			UpdatedAt: e.UpdatedAt,
		},
		UserID:        e.UserID,
		TransactionID: e.TransactionID,
		Satoshis:      e.Satoshis,
		KeyOffset:     e.KeyOffset,
		IsRedeemed:    e.IsRedeemed,
		LockingScript: e.LockingScript,
	}
}

func (s *SyncCommission) upsertCommissionModel(tx *gorm.DB, e *entity.Commission, model models.Commission) (isNew bool, commissionID uint, err error) {
	var existing models.Commission
	existsErr := tx.Model(&models.Commission{}).
		Select("id, updated_at").
		Where("user_id = ? AND transaction_id = ?", e.UserID, e.TransactionID).
		First(&existing).Error

	if existsErr == nil {
		id, updateErr := s.updateCommissionIfNewer(tx, model, existing)
		return false, id, updateErr
	}
	if !errors.Is(existsErr, gorm.ErrRecordNotFound) {
		return false, 0, fmt.Errorf("failed to lookup existing commission: %w", existsErr)
	}

	id, createErr := s.createCommission(tx, model)
	if createErr != nil {
		return false, 0, createErr
	}
	return true, id, nil
}

func (s *SyncCommission) updateCommissionIfNewer(tx *gorm.DB, model models.Commission, existing models.Commission) (uint, error) {
	if !model.UpdatedAt.After(existing.UpdatedAt) {
		return existing.ID, nil
	}

	// TS mergeExisting only updates isRedeemed (plus updated_at).
	updateTx := tx.Model(&models.Commission{}).
		Where("id = ? AND updated_at < ?", existing.ID, model.UpdatedAt).
		Updates(map[string]any{
			"is_redeemed": model.IsRedeemed,
			"updated_at":  model.UpdatedAt,
		})
	if updateTx.Error != nil {
		return 0, fmt.Errorf("failed to update commission: %w", updateTx.Error)
	}
	return existing.ID, nil
}

func (s *SyncCommission) createCommission(tx *gorm.DB, model models.Commission) (uint, error) {
	if err := tx.Create(&model).Error; err != nil {
		return 0, fmt.Errorf("failed to create commission: %w", err)
	}
	if model.ID == 0 {
		return 0, fmt.Errorf("commission ID is zero after creation")
	}
	return model.ID, nil
}

func (s *SyncCommission) mapModelToTableCommission(model *models.Commission) *wdk.TableCommission {
	return &wdk.TableCommission{
		CreatedAt:     model.CreatedAt,
		UpdatedAt:     model.UpdatedAt,
		CommissionID:  model.ID,
		UserID:        model.UserID,
		TransactionID: model.TransactionID,
		Satoshis:      must.ConvertToInt64FromUnsigned(model.Satoshis),
		KeyOffset:     model.KeyOffset,
		IsRedeemed:    model.IsRedeemed,
		LockingScript: primitives.ExplicitByteArray(model.LockingScript),
	}
}
