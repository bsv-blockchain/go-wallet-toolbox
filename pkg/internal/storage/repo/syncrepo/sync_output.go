package syncrepo

import (
	"context"
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/genquery"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/scopes"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/slices"
	"github.com/go-softwarelab/common/pkg/to"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SyncOutput struct {
	db    *gorm.DB
	query *genquery.Query
}

func NewSyncOutput(db *gorm.DB, query *genquery.Query) *SyncOutput {
	return &SyncOutput{db: db, query: query}
}

type OutputReadModel struct {
	models.Output
	BasketNumID *int `gorm:"column:basket_num_id"`
}

func (s *SyncOutput) tableName() string {
	return s.query.Output.TableName()
}

func (s *SyncOutput) FindOutputsForSync(ctx context.Context, userID int, opts ...queryopts.Options) ([]*wdk.TableOutput, error) {
	const basketStringIDClause = "CONCAT(user_id, '.', basket_name)"
	var resultModels []*OutputReadModel

	queryopts.ModifyOptions(opts, func(options *queryopts.Options) {
		if options.Since != nil && options.Since.TableName == "" {
			// Prevent from an issue with ambiguous created_at column
			options.Since.TableName = s.tableName()
		}
	})
	filters := append(scopes.FromQueryOpts(opts), scopes.UserID(userID))

	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Make sure all numeric IDs of OutputBaskets needed by user's outputs are present in the numeric ID lookup table.
		err := upsertNumericIDLookup(ctx, s.db, tx, s.query, func(db *gorm.DB) *gorm.DB {
			return db.
				Select(fmt.Sprintf("?, %s", basketStringIDClause), s.query.OutputBasket.TableName()).
				Scopes(filters...).
				Where("basket_name IS NOT NULL").
				Find(&models.Output{})
		})
		if err != nil {
			return err
		}

		err = tx.WithContext(ctx).
			Model(&models.Output{}).
			Select(fmt.Sprintf("%s.*, num.num_id as basket_num_id", s.tableName())).
			Scopes(filters...).
			Scopes(joinWithNumericIDLookupScope(s.query, basketStringIDClause, s.query.OutputBasket.TableName(), clause.LeftJoin)).
			Preload("Transaction", func(db *gorm.DB) *gorm.DB {
				return db.Select("id, tx_id")
			}).
			Find(&resultModels).Error
		if err != nil {
			return fmt.Errorf("failed to find outputs for sync: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("transaction failed: %w", err)
	}

	return slices.Map(resultModels, s.mapModelToTableOutput), nil
}

func (s *SyncOutput) UpsertOutputForSync(ctx context.Context, entity *entity.Output) (isNew bool, outputID uint, err error) {
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var transaction models.Transaction
		err = tx.Model(models.Transaction{}).
			Select("status").
			Where("id = ?", entity.TransactionID).
			First(&transaction).Error
		if err != nil {
			return fmt.Errorf("failed to check known transaction: %w", err)
		}

		utxoStatus := s.utxoStatusByTxStatus(transaction.Status)

		isNew, outputID, err = s.upsertOutput(tx, entity)
		if err != nil {
			return fmt.Errorf("failed to upsert output: %w", err)
		}

		if entity.UserUTXO != nil && utxoStatus != wdk.UTXOStatusUnknown {
			entity.UserUTXO.OutputID = outputID
			entity.UserUTXO.Status = utxoStatus

			err = s.upsertUserUTXO(tx, entity.UserUTXO)
			if err != nil {
				return fmt.Errorf("failed to upsert user UTXO: %w", err)
			}
		}

		return nil
	})

	if err != nil {
		return false, 0, fmt.Errorf("transaction failed: %w", err)
	}

	return isNew, outputID, nil
}

func (s *SyncOutput) upsertOutput(tx *gorm.DB, entity *entity.Output) (isNew bool, outputID uint, err error) {
	model := models.Output{
		Model: gorm.Model{
			CreatedAt: entity.CreatedAt,
			UpdatedAt: entity.UpdatedAt,
		},
		UserID:             entity.UserID,
		TransactionID:      entity.TransactionID,
		SpentBy:            entity.SpentBy,
		Satoshis:           entity.Satoshis,
		Description:        entity.Description,
		Vout:               entity.Vout,
		LockingScript:      entity.LockingScript,
		CustomInstructions: entity.CustomInstructions,
		DerivationPrefix:   entity.DerivationPrefix,
		DerivationSuffix:   entity.DerivationSuffix,
		BasketName:         entity.BasketName,
		Spendable:          entity.Spendable,
		Change:             entity.Change,
		Purpose:            entity.Purpose,
		Type:               entity.Type,
		SenderIdentityKey:  entity.SenderIdentityKey,
	}

	updateTx := tx.Model(&model).
		Clauses(clause.Returning{Columns: []clause.Column{{Name: "id"}}}).
		Where("user_id = ? AND transaction_id = ? AND vout = ?", model.UserID, model.TransactionID, model.Vout).
		Select("*").
		Updates(&model)

	// NOTE: We use `Select("*")` with `Updates()` to ensure that all fields are updated, including those that might be zero values (e.g., BasketName for relinquished outputs).

	if updateTx.Error != nil {
		err = fmt.Errorf("failed to update output: %w", updateTx.Error)
		return
	}

	if updateTx.RowsAffected > 0 {
		resultTxModel := models.Output{}
		if err = updateTx.Scan(&resultTxModel).Error; err != nil {
			err = fmt.Errorf("failed to scan updated output: %w", err)
			return
		}

		if resultTxModel.ID == 0 {
			err = fmt.Errorf("output ID is zero after update, this should not happen")
			return
		}

		outputID = resultTxModel.ID
		return
	}

	err = tx.Create(&model).Error
	if err != nil {
		err = fmt.Errorf("failed to create output: %w", err)
		return
	}

	if model.ID == 0 {
		err = fmt.Errorf("output ID is zero after update, this should not happen")
		return
	}

	isNew = true
	outputID = model.ID

	return
}

func (s *SyncOutput) upsertUserUTXO(tx *gorm.DB, userUTXO *entity.UserUTXO) error {
	model := &models.UserUTXO{
		UserID:             userUTXO.UserID,
		OutputID:           userUTXO.OutputID,
		BasketName:         userUTXO.BasketName,
		Satoshis:           userUTXO.Satoshis,
		EstimatedInputSize: userUTXO.EstimatedInputSize,
		CreatedAt:          userUTXO.CreatedAt,
		ReservedByID:       userUTXO.ReservedByID,
		UTXOStatus:         userUTXO.Status,
	}

	updateTx := tx.Model(&models.UserUTXO{}).
		Where("user_id = ? AND output_id = ?", userUTXO.UserID, userUTXO.OutputID).
		Select("*").
		Updates(model)

	if updateTx.Error != nil {
		return fmt.Errorf("failed to update user UTXO: %w", updateTx.Error)
	}

	if updateTx.RowsAffected > 0 {
		return nil
	}

	err := tx.Create(model).Error
	if err != nil {
		return fmt.Errorf("failed to create user UTXO: %w", err)
	}

	return nil
}

func (s *SyncOutput) mapModelToTableOutput(model *OutputReadModel) *wdk.TableOutput {
	return &wdk.TableOutput{
		CreatedAt:          model.CreatedAt,
		UpdatedAt:          model.UpdatedAt,
		OutputID:           model.ID,
		UserID:             model.UserID,
		TransactionID:      model.TransactionID,
		Spendable:          model.Spendable,
		Change:             model.Change,
		OutputDescription:  model.Description,
		Vout:               model.Vout,
		Satoshis:           model.Satoshis,
		ProvidedBy:         model.ProvidedBy,
		Purpose:            model.Purpose,
		Type:               model.Type,
		TxID:               to.IfThen(model.Transaction != nil, model.Transaction.TxID).ElseThen(nil),
		DerivationPrefix:   model.DerivationPrefix,
		DerivationSuffix:   model.DerivationSuffix,
		CustomInstructions: model.CustomInstructions,
		LockingScript:      model.LockingScript,
		SenderIdentityKey:  model.SenderIdentityKey,
		BasketID:           model.BasketNumID,
		SpentBy:            model.SpentBy,
	}
}

func (s *SyncOutput) utxoStatusByTxStatus(txStatus wdk.TxStatus) wdk.UTXOStatus {
	switch txStatus {
	case wdk.TxStatusCompleted:
		return wdk.UTXOStatusMined
	case wdk.TxStatusSending:
		return wdk.UTXOStatusSending
	case wdk.TxStatusUnproven:
		return wdk.UTXOStatusUnproven
	case wdk.TxStatusFailed, wdk.TxStatusUnprocessed, wdk.TxStatusUnsigned, wdk.TxStatusNoSend, wdk.TxStatusNonFinal, wdk.TxStatusUnfail:
		fallthrough
	default:
		return wdk.UTXOStatusUnknown
	}
}
