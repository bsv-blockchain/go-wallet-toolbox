package syncrepo

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-softwarelab/common/pkg/slices"
	"github.com/go-softwarelab/common/pkg/to"
	"gorm.io/gorm"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/genquery"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/scopes"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type SyncOutput struct {
	db    *gorm.DB
	query *genquery.Query
}

func NewSyncOutput(db *gorm.DB, query *genquery.Query) *SyncOutput {
	return &SyncOutput{db: db, query: query}
}

func (s *SyncOutput) tableName() string {
	return s.query.Output.TableName()
}

func (s *SyncOutput) FindOutputsForSync(ctx context.Context, userID int, opts ...queryopts.Options) ([]*wdk.TableOutput, error) {
	var resultModels []*models.Output

	queryopts.ModifyOptions(opts, func(options *queryopts.Options) {
		if options.Since != nil && options.Since.TableName == "" {
			options.Since.TableName = s.tableName()
		}
	})
	filters := append(scopes.FromQueryOpts(opts), scopes.UserID(userID))

	err := s.db.WithContext(ctx).
		Model(&models.Output{}).
		Scopes(filters...).
		Preload("Transaction", func(db *gorm.DB) *gorm.DB {
			return db.Select("transactionId, txid")
		}).
		Find(&resultModels).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find outputs for sync: %w", err)
	}

	return slices.Map(resultModels, s.mapModelToTableOutput), nil
}

func (s *SyncOutput) UpsertOutputForSync(ctx context.Context, entity *entity.Output) (isNew bool, outputID uint, err error) {
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var transaction models.Transaction
		err = tx.Model(models.Transaction{}).
			Select("status").
			Where("transactionId = ?", entity.TransactionID).
			First(&transaction).Error
		if err != nil {
			return fmt.Errorf("failed to check known transaction: %w", err)
		}
		isNew, outputID, err = s.upsertOutput(tx, entity)
		if err != nil {
			return fmt.Errorf("failed to upsert output: %w", err)
		}

		// user_utxo is deleted in the new schema, we no longer need to upsert user_utxo here.
		// wait, is UserUTXO still populated in sync? The task says C-repo rewritten utxos selection,
		// and Wave 1 deleted user_utxo.go. So we don't need to upsert UserUTXO here!

		return nil
	})
	if err != nil {
		return false, 0, fmt.Errorf("transaction failed: %w", err)
	}

	return isNew, outputID, nil
}

func (s *SyncOutput) upsertOutput(tx *gorm.DB, entity *entity.Output) (isNew bool, outputID uint, err error) {
	var basketID *uint = entity.BasketID

	model := models.Output{
		Timestamps: models.Timestamps{
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
		BasketID:           basketID,
		Spendable:          entity.Spendable,
		Change:             entity.Change,
		Purpose:            entity.Purpose,
		Type:               entity.Type,
		SenderIdentityKey:  entity.SenderIdentityKey,
	}

	// BRC-40 stale-chunk guard
	var existing models.Output
	existsErr := tx.Model(&models.Output{}).
		Select("outputId, updated_at").
		Where("userId = ? AND transactionId = ? AND vout = ?", model.UserID, model.TransactionID, model.Vout).
		First(&existing).Error

	if existsErr == nil {
		if !model.UpdatedAt.After(existing.UpdatedAt) {
			outputID = existing.OutputID
			return isNew, outputID, nil
		}

		updateTx := tx.Model(&model).
			Where("outputId = ? AND updated_at < ?", existing.OutputID, model.UpdatedAt).
			Select("*").
			Updates(&model)

		if updateTx.Error != nil {
			err = fmt.Errorf("failed to update output: %w", updateTx.Error)
			return isNew, outputID, err
		}

		outputID = existing.OutputID
		return isNew, outputID, nil
	}

	if !errors.Is(existsErr, gorm.ErrRecordNotFound) {
		err = fmt.Errorf("failed to lookup existing output: %w", existsErr)
		return isNew, outputID, err
	}

	err = tx.Create(&model).Error
	if err != nil {
		err = fmt.Errorf("failed to create output: %w", err)
		return isNew, outputID, err
	}

	if model.OutputID == 0 {
		err = fmt.Errorf("output ID is zero after update, this should not happen")
		return isNew, outputID, err
	}

	isNew = true
	outputID = model.OutputID

	return isNew, outputID, err
}

func (s *SyncOutput) mapModelToTableOutput(model *models.Output) *wdk.TableOutput {
	var basketID *int
	if model.BasketID != nil {
		basketID = to.Ptr(int(*model.BasketID))
	}
	return &wdk.TableOutput{
		CreatedAt:          model.CreatedAt,
		UpdatedAt:          model.UpdatedAt,
		OutputID:           model.OutputID,
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
		BasketID:           basketID,
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
