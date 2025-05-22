package repo

import (
	"context"
	"errors"
	"fmt"

	models2 "github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/database/scopes"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/txutils"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/go-softwarelab/common/pkg/is"
	"github.com/go-softwarelab/common/pkg/must"
	"github.com/go-softwarelab/common/pkg/slices"
	"github.com/go-softwarelab/common/pkg/to"
	"gorm.io/gorm"
)

type Transactions struct {
	db *gorm.DB
}

func NewTransactions(db *gorm.DB) *Transactions {
	return &Transactions{db: db}
}

func (txs *Transactions) CreateTransaction(ctx context.Context, newTx *entity.NewTx) error {
	model, err := txs.toTransactionModel(newTx)
	if err != nil {
		return err
	}

	err = txs.db.WithContext(ctx).Transaction(func(tx *gorm.DB) (err error) {
		err = txs.connectOutputsWithBaskets(tx, newTx, model)
		if err != nil {
			return err
		}

		if err = txs.markReservedOutputsAsNotSpendable(tx, newTx.UserID, newTx.ReservedOutputIDs); err != nil {
			return err
		}

		return tx.Create(model).Error
	})
	if err != nil {
		return fmt.Errorf("failed to create transaction: %w", err)
	}
	return nil
}

func (txs *Transactions) toTransactionModel(newTx *entity.NewTx) (*models2.Transaction, error) {
	outputs, err := slices.MapOrError(newTx.Outputs, func(output *entity.NewOutput) (*models2.Output, error) {
		return txs.makeNewOutput(newTx.UserID, output)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create outputs: %w", err)
	}
	model := &models2.Transaction{
		UserID:      newTx.UserID,
		Status:      newTx.Status,
		Reference:   newTx.Reference,
		IsOutgoing:  newTx.IsOutgoing,
		Satoshis:    newTx.Satoshis,
		Description: newTx.Description,
		Version:     newTx.Version,
		LockTime:    newTx.LockTime,
		InputBeef:   newTx.InputBeef,
		TxID:        newTx.TxID,
		Labels: slices.Map(newTx.Labels, func(label primitives.StringUnder300) *models2.Label {
			return &models2.Label{
				Name:   string(label),
				UserID: newTx.UserID,
			}
		}),
		ReservedUtxos: slices.Map(newTx.ReservedOutputIDs, func(reservedOutputID uint) *models2.UserUTXO {
			return &models2.UserUTXO{
				UserID:   newTx.UserID,
				OutputID: reservedOutputID,
			}
		}),
		Outputs: outputs,
	}

	return model, nil
}

func (txs *Transactions) connectOutputsWithBaskets(tx *gorm.DB, newTx *entity.NewTx, model *models2.Transaction) error {
	basketMaker := newCachedBasketMaker(tx, newTx.UserID)
	for _, out := range model.Outputs {
		if out.Basket == nil || out.Basket.Name == "" {
			continue
		}
		basketID, err := basketMaker.findOrCreate(tx, out.Basket.Name, wdk.DefaultNumberOfDesiredUTXOs, wdk.DefaultMinimumDesiredUTXOValue)
		if err != nil || basketID == nil {
			return fmt.Errorf("failed to find or create output basket: %w", err)
		}

		out.BasketID = basketID
		out.Basket = nil
		if out.UserUTXO != nil {
			out.UserUTXO.BasketID = *basketID
		}
	}
	return nil
}

func (txs *Transactions) makeNewOutput(userID int, output *entity.NewOutput) (*models2.Output, error) {
	out := models2.Output{
		Vout:               output.Vout,
		UserID:             userID,
		Satoshis:           output.Satoshis.Int64(),
		Spendable:          output.Spendable,
		Change:             output.Change,
		ProvidedBy:         string(output.ProvidedBy),
		Description:        output.Description,
		Purpose:            output.Purpose,
		Type:               string(output.Type),
		DerivationPrefix:   output.DerivationPrefix,
		DerivationSuffix:   output.DerivationSuffix,
		LockingScript:      (*string)(output.LockingScript),
		CustomInstructions: output.CustomInstructions,
		SenderIdentityKey:  output.SenderIdentityKey,
	}

	if output.Basket != nil && *output.Basket != "" {
		// This won't create a new basket, the name is just passed for further processing (see connectOutputsWithBaskets())
		out.Basket = &models2.OutputBasket{
			Name: *output.Basket,
		}
	}

	if out.Spendable && out.Change {
		if is.EmptyString(output.Basket) {
			return nil, fmt.Errorf("basket not provided for change output")
		}
		if out.Satoshis == 0 {
			return nil, fmt.Errorf("change output with zero satoshis")
		}
		sats, err := to.UInt64(out.Satoshis)
		if err != nil {
			return nil, fmt.Errorf("failed to convert satoshis to uint64: %w", err)
		}

		out.UserUTXO = &models2.UserUTXO{
			UserID:             userID,
			Satoshis:           sats,
			EstimatedInputSize: txutils.EstimatedInputSizeByType(output.Type),
		}
	}
	return &out, nil
}

func (txs *Transactions) markReservedOutputsAsNotSpendable(tx *gorm.DB, userID int, outputIDs []uint) error {
	if len(outputIDs) == 0 {
		return nil
	}

	err := tx.Model(&models2.Output{}).
		Where("id IN ?", outputIDs).
		Where("user_id = ?", userID).
		Update("spendable", false).Error
	if err != nil {
		return fmt.Errorf("failed to mark reserved outputs as not spendable: %w", err)
	}
	return nil
}

func (txs *Transactions) FindTransactionByUserIDAndTxID(ctx context.Context, userID int, txID string) (*wdk.TableTransaction, error) {
	var transaction models2.Transaction
	err := txs.db.WithContext(ctx).Scopes(scopes.UserID(userID)).Where("tx_id = ?", txID).First(&transaction).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find transaction: %w", err)
	}

	return txs.mapModelToTableTransaction(&transaction), nil

}

func (txs *Transactions) FindTransactionByReference(ctx context.Context, userID int, reference string) (*wdk.TableTransaction, error) {
	transaction := models2.Transaction{}
	err := txs.db.WithContext(ctx).
		Scopes(scopes.UserID(userID)).
		Where("reference = ?", reference).
		First(&transaction).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find transaction by reference: %w", err)
	}

	return txs.mapModelToTableTransaction(&transaction), nil
}

func (txs *Transactions) SpendTransaction(
	ctx context.Context,
	updatedTx entity.UpdatedTx,
	historyNote string,
	historyAttrs map[string]any,
) error {
	err := txs.db.WithContext(ctx).Transaction(func(tx *gorm.DB) (err error) {
		err = tx.Model(models2.Transaction{}).
			Scopes(scopes.UserID(updatedTx.UserID)).
			Where("id = ?", updatedTx.TransactionID).
			Updates(map[string]any{
				"tx_id":      updatedTx.TxID,
				"input_beef": nil, // input_beef per user's transaction won't be needed anymore; it is moved to the ProvenTxReq (storage-wide)
				"status":     updatedTx.TxStatus,
			}).Error
		if err != nil {
			return err
		}

		err = tx.Delete(models2.UserUTXO{}, "reserved_by_id = ?", updatedTx.TransactionID).Error
		if err != nil {
			return err
		}

		err = makeOutputsSpendable(tx, updatedTx)
		if err != nil {
			return err
		}

		return upsertProvenTxReq(tx, &entity.UpsertProvenTxReq{
			TxID:      updatedTx.TxID,
			Status:    updatedTx.ReqTxStatus,
			RawTx:     updatedTx.RawTx,
			InputBeef: updatedTx.InputBeef,
		}, historyNote, historyAttrs)
	})
	if err != nil {
		return fmt.Errorf("failed to update transaction: %w", err)
	}
	return nil
}

func makeOutputsSpendable(tx *gorm.DB, updatedTx entity.UpdatedTx) error {
	var changeOutputs []*models2.Output
	err := tx.
		Model(&models2.Transaction{
			Model: gorm.Model{
				ID: updatedTx.TransactionID,
			},
		}).
		Association("Outputs").
		Find(&changeOutputs, "basket_id IS NOT NULL AND change = ? AND satoshis > 0 AND spent_by IS NULL", true)
	if err != nil {
		return fmt.Errorf("failed to find transaction outputs: %w", err)
	}

	if len(changeOutputs) == 0 {
		return nil
	}

	for _, output := range changeOutputs {
		output.Spendable = true
		output.LockingScript, err = updatedTx.GetLockingScript(output.Vout)
		if err != nil {
			return fmt.Errorf("failed to get locking script: %w", err)
		}
	}

	err = tx.Save(changeOutputs).Error
	if err != nil {
		return fmt.Errorf("failed to save change outputs: %w", err)
	}

	newUTXOs := slices.Map(changeOutputs, func(output *models2.Output) *models2.UserUTXO {
		return &models2.UserUTXO{
			UserID:             updatedTx.UserID,
			OutputID:           output.ID,
			BasketID:           *output.BasketID,
			Satoshis:           must.ConvertToUInt64(output.Satoshis),
			EstimatedInputSize: txutils.EstimatedInputSizeByType(wdk.OutputType(output.Type)),
		}
	})

	err = tx.Create(newUTXOs).Error
	if err != nil {
		return fmt.Errorf("failed to create new UTXOs: %w", err)
	}
	return nil
}

func (txs *Transactions) UpdateTransactionStatusForTxID(
	ctx context.Context,
	txID string,
	txStatus wdk.TxStatus,
	provenTxReqStatus wdk.ProvenTxReqStatus,
	historyNote string,
	historyAttrs map[string]any,
) error {
	err := txs.db.WithContext(ctx).Transaction(func(tx *gorm.DB) (err error) {
		err = updateTransactionStatus(tx, txID, txStatus)
		if err != nil {
			return err
		}

		return updateProvenTxStatus(tx, txID, provenTxReqStatus, historyNote, historyAttrs)
	})
	if err != nil {
		return fmt.Errorf("failed to update transaction: %w", err)
	}
	return nil
}

func updateTransactionStatus(tx *gorm.DB, txID string, txStatus wdk.TxStatus) error {
	return tx.Model(models2.Transaction{}).
		Where("tx_id = ?", txID).
		Updates(map[string]any{
			"status": txStatus,
		}).Error
}

func (txs *Transactions) mapModelToTableTransaction(model *models2.Transaction) *wdk.TableTransaction {
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
		Version:       to.Ptr(model.Version),
		LockTime:      to.Ptr(model.LockTime),
		TxID:          model.TxID,
		InputBEEF:     model.InputBeef,
	}
}
