package syncrepo

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-softwarelab/common/pkg/slices"
	"gorm.io/gorm"

	pkgentity "github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/genquery"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/scopes"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

type SyncTransaction struct {
	db    *gorm.DB
	query *genquery.Query
}

func NewSyncTransaction(db *gorm.DB, query *genquery.Query) *SyncTransaction {
	return &SyncTransaction{db: db, query: query}
}

func (s *SyncTransaction) tableName() string {
	return s.query.Transaction.TableName()
}

func (s *SyncTransaction) FindTransactionsForSync(ctx context.Context, userID int, opts ...queryopts.Options) ([]*wdk.TableTransaction, error) {
	queryopts.ModifyOptions(opts, func(options *queryopts.Options) {
		if options.Since != nil && options.Since.TableName == "" {
			options.Since.TableName = s.tableName()
		}
	})
	filters := append(scopes.FromQueryOpts(opts), scopes.UserID(userID))

	var resultModels []*models.Transaction

	err := s.db.WithContext(ctx).
		Model(&models.Transaction{}).
		Scopes(filters...).
		Find(&resultModels).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find transactions for sync: %w", err)
	}

	return slices.Map(resultModels, s.mapModelToTableTransaction), nil
}

func (s *SyncTransaction) UpsertTransactionForSync(ctx context.Context, entity *pkgentity.Transaction) (isNew bool, transactionID uint, err error) {
	model := models.Transaction{
		Timestamps: models.Timestamps{
			CreatedAt: entity.CreatedAt,
			UpdatedAt: entity.UpdatedAt,
		},
		UserID:      entity.UserID,
		Status:      entity.Status,
		Reference:   entity.Reference,
		IsOutgoing:  entity.IsOutgoing,
		Satoshis:    entity.Satoshis,
		Description: entity.Description,
		Version:     entity.Version,
		LockTime:    entity.LockTime,
		TxID:        entity.TxID,
		InputBeef:   entity.InputBEEF,
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing models.Transaction
		existsErr := tx.Model(&models.Transaction{}).
			Select("transactionId, updated_at").
			Scopes(scopes.UserID(entity.UserID)).
			Where("reference = ?", entity.Reference).
			First(&existing).Error

		if existsErr == nil {
			if !model.UpdatedAt.After(existing.UpdatedAt) {
				transactionID = existing.TransactionID
				return nil
			}

			updateTx := tx.Model(&models.Transaction{}).
				Where("transactionId = ? AND updated_at < ?", existing.TransactionID, model.UpdatedAt).
				Updates(model)

			if updateTx.Error != nil {
				return fmt.Errorf("failed to update transaction: %w", updateTx.Error)
			}

			transactionID = existing.TransactionID
			return nil
		}

		if !errors.Is(existsErr, gorm.ErrRecordNotFound) {
			return fmt.Errorf("failed to lookup existing transaction: %w", existsErr)
		}

		if err = tx.Create(&model).Error; err != nil {
			return fmt.Errorf("failed to create transaction: %w", err)
		}

		if model.TransactionID == 0 {
			return fmt.Errorf("transaction ID is zero after creation, this should not happen")
		}

		isNew = true
		transactionID = model.TransactionID

		return nil
	})
	if err != nil {
		return false, 0, fmt.Errorf("transaction failed: %w", err)
	}

	return isNew, transactionID, nil
}

func (s *SyncTransaction) mapModelToTableTransaction(model *models.Transaction) *wdk.TableTransaction {
	var provenTxID *int
	if model.ProvenTxID != nil {
		id := int(*model.ProvenTxID)
		provenTxID = &id
	}

	return &wdk.TableTransaction{
		CreatedAt:     model.CreatedAt,
		UpdatedAt:     model.UpdatedAt,
		TransactionID: model.TransactionID,
		UserID:        model.UserID,
		Status:        model.Status,
		Reference:     primitives.Base64String(model.Reference),
		IsOutgoing:    model.IsOutgoing,
		Satoshis:      model.Satoshis,
		Description:   model.Description,
		Version:       model.Version,
		LockTime:      model.LockTime,
		TxID:          model.TxID,
		InputBEEF:     model.InputBeef,

		ProvenTxID: provenTxID,
	}
}
