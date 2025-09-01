package syncrepo

import (
	"context"
	"fmt"

	pkgentity "github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/genquery"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/scopes"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/go-softwarelab/common/pkg/slices"
	"github.com/go-softwarelab/common/pkg/to"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SyncTransaction struct {
	db    *gorm.DB
	query *genquery.Query
}

func NewSyncTransaction(db *gorm.DB, query *genquery.Query) *SyncTransaction {
	return &SyncTransaction{db: db, query: query}
}

type TransactionWithKnownTx struct {
	models.Transaction
	KnownTxNumID *int `gorm:"column:num_id"`
	BlockHeight  *uint32
}

func (s *SyncTransaction) tableName() string {
	return s.query.Transaction.TableName()
}

func (s *SyncTransaction) FindTransactionsForSync(ctx context.Context, userID int, opts ...queryopts.Options) ([]*wdk.TableTransaction, error) {
	queryopts.ModifyOptions(opts, func(options *queryopts.Options) {
		if options.Since != nil && options.Since.TableName == "" {
			// Prevent from an issue with ambiguous created_at column
			options.Since.TableName = s.tableName()
		}
	})
	filters := append(scopes.FromQueryOpts(opts), scopes.UserID(userID))

	var resultModels []*TransactionWithKnownTx

	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Make sure all numeric IDs of KnownTxs needed by user's transactions are present in the numeric ID lookup table.
		err := upsertNumericIDLookup(ctx, s.db, tx, s.query, func(db *gorm.DB) *gorm.DB {
			return db.
				Select("?, tx_id", s.query.KnownTx.TableName()).
				Scopes(filters...).
				Where("tx_id IS NOT NULL").
				Find(&models.Transaction{})
		})
		if err != nil {
			return err
		}

		err = tx.WithContext(ctx).
			Model(&models.Transaction{}).
			Select(fmt.Sprintf("%s.*, num.num_id, known_tx.block_height", s.tableName())).
			Scopes(joinWithNumericIDLookupScope(s.query, fmt.Sprintf("%s.tx_id", s.tableName()), s.query.KnownTx.TableName(), clause.LeftJoin)).
			Joins(fmt.Sprintf("LEFT JOIN %s as known_tx ON known_tx.tx_id = %s.tx_id", s.query.KnownTx.TableName(), s.tableName())).
			Scopes(filters...).
			Find(&resultModels).Error
		if err != nil {
			return fmt.Errorf("failed to find proven tx requests for sync: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("transaction failed: %w", err)
	}

	return slices.Map(resultModels, s.mapModelToTableTransaction), nil
}

func (s *SyncTransaction) UpsertTransactionForSync(ctx context.Context, entity *pkgentity.Transaction) (isNew bool, transactionID uint, err error) {
	model := models.Transaction{
		Model: gorm.Model{
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
		updateTx := tx.Model(&models.Transaction{}).
			Clauses(clause.Returning{Columns: []clause.Column{{Name: "id"}}}).
			Scopes(scopes.UserID(entity.UserID)).
			Where("reference = ?", entity.Reference).
			Updates(model)

		if updateTx.Error != nil {
			return fmt.Errorf("failed to update transaction: %w", updateTx.Error)
		}

		if updateTx.RowsAffected > 0 {
			resultTxModel := models.Transaction{}
			if err = updateTx.Scan(&resultTxModel).Error; err != nil {
				return fmt.Errorf("failed to scan updated transaction: %w", err)
			}

			if resultTxModel.ID == 0 {
				return fmt.Errorf("transaction ID is zero after update, this should not happen")
			}

			transactionID = resultTxModel.ID
			return nil
		}

		err := tx.Create(&model).Error
		if err != nil {
			return fmt.Errorf("failed to create transaction: %w", err)
		}

		if model.ID == 0 {
			return fmt.Errorf("transaction ID is zero after creation, this should not happen")
		}

		isNew = true
		transactionID = model.ID

		return nil
	})

	if err != nil {
		return false, 0, fmt.Errorf("transaction failed: %w", err)
	}

	return isNew, transactionID, nil
}

func (s *SyncTransaction) mapModelToTableTransaction(model *TransactionWithKnownTx) *wdk.TableTransaction {
	return &wdk.TableTransaction{
		CreatedAt:     model.CreatedAt,
		UpdatedAt:     model.UpdatedAt,
		TransactionID: model.ID,
		UserID:        model.UserID,
		Status:        model.Status,
		Reference:     primitives.Base64String(model.Reference),
		IsOutgoing:    model.IsOutgoing,
		Satoshis:      model.Satoshis,
		Description:   model.Description,
		Version:       &model.Version,
		LockTime:      &model.LockTime,
		TxID:          model.TxID,
		InputBEEF:     model.InputBeef,

		//NOTE: ProvenTxID is set only if the transaction is known to be mined (has a numeric ID in the KnownTx table).
		ProvenTxID: to.IfThen(model.BlockHeight != nil, model.KnownTxNumID).ElseThen(nil),
	}
}
