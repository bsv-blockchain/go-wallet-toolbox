package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"iter"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/genquery"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/scopes"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/history"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/txutils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/go-softwarelab/common/pkg/is"
	"github.com/go-softwarelab/common/pkg/seqerr"
	"github.com/go-softwarelab/common/pkg/slices"
	"github.com/go-softwarelab/common/pkg/to"
	"gorm.io/gorm"
)

type abortStep struct {
	name string
	fn   func(*Transactions, *gorm.DB, uint) error
}

var abortTransactionSteps = []abortStep{
	{"release_reserved_utxos", (*Transactions).releaseReservedUTXOs},
	{"check_if_any_output_is_spent", (*Transactions).checkIfAnyOutputIsSpent},
	{"release_outputs_reserved", (*Transactions).releaseOutputsReservedByTransaction},
	{"delete_outputs", (*Transactions).deleteOutputsByTransactionID},
	{"update_transaction_status", func(txs *Transactions, tx *gorm.DB, transactionID uint) error {
		return txs.updateTransactionStatusByID(tx, transactionID, wdk.TxStatusFailed)
	}},
}

type Transactions struct {
	query *genquery.Query
	db    *gorm.DB
}

func NewTransactions(db *gorm.DB, query *genquery.Query) *Transactions {
	return &Transactions{db: db, query: query}
}

func (txs *Transactions) CreateTransaction(ctx context.Context, newTx *entity.NewTx) error {
	model, err := txs.toTransactionModel(newTx)
	if err != nil {
		return err
	}

	err = txs.db.WithContext(ctx).Transaction(func(tx *gorm.DB) (err error) {
		err = txs.connectOutputsWithBaskets(tx, newTx, model)
		if err != nil {
			return fmt.Errorf("failed to connect outputs with baskets: %w", err)
		}

		if err = tx.Create(model).Error; err != nil {
			return fmt.Errorf("failed to create new transaction model: %w", err)
		}

		if err = txs.markReservedOutputsAsNotSpendable(tx, model.ID, newTx.UserID, newTx.ReservedOutputIDs); err != nil {
			return fmt.Errorf("failed to mark reserved outputs as not spendable: %w", err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to create transaction: %w", err)
	}
	return nil
}

func (txs *Transactions) toTransactionModel(newTx *entity.NewTx) (*models.Transaction, error) {
	outputs, err := slices.MapOrError(newTx.Outputs, func(output *entity.NewOutput) (*models.Output, error) {
		return txs.makeNewOutput(newTx.UserID, output, newTx.UTXOStatus)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create outputs: %w", err)
	}
	model := &models.Transaction{
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
		Labels: slices.Map(newTx.Labels, func(label primitives.StringUnder300) *models.Label {
			return &models.Label{
				Name:   string(label),
				UserID: newTx.UserID,
			}
		}),
		// TODO: verify if this won't blow up for not created UTXOs (when we're using noSendChange - which are not in UTXO table)
		ReservedUtxos: slices.Map(newTx.ReservedOutputIDs, func(reservedOutputID uint) *models.UserUTXO {
			return &models.UserUTXO{
				UserID:   newTx.UserID,
				OutputID: reservedOutputID,
			}
		}),
		Outputs: outputs,
		Commission: to.If(newTx.Commission != nil, func() *models.Commission {
			return &models.Commission{
				UserID:        newTx.UserID,
				Satoshis:      newTx.Commission.Satoshis,
				KeyOffset:     newTx.Commission.KeyOffset,
				IsRedeemed:    newTx.Commission.IsRedeemed,
				LockingScript: newTx.Commission.LockingScript,
			}
		}).ElseThen(nil),
	}

	return model, nil
}

func (txs *Transactions) connectOutputsWithBaskets(tx *gorm.DB, newTx *entity.NewTx, model *models.Transaction) error {
	basketMaker := newCachedBasketMaker(tx, newTx.UserID)
	for _, out := range model.Outputs {
		if out.BasketName == nil || *out.BasketName == "" {
			continue
		}
		err := basketMaker.createIfNotExist(tx, *out.BasketName, wdk.NonChangeBasketConfiguration.NumberOfDesiredUTXOs, wdk.NonChangeBasketConfiguration.MinimumDesiredUTXOValue)
		if err != nil {
			return fmt.Errorf("failed to find or create output basket: %w", err)
		}

		if out.UserUTXO != nil {
			out.UserUTXO.BasketName = *out.BasketName
		}
	}
	return nil
}

func (txs *Transactions) makeNewOutput(userID int, output *entity.NewOutput, utxoStatus wdk.UTXOStatus) (*models.Output, error) {
	tags := slices.Map(output.Tags, func(tag string) *models.Tag {
		return &models.Tag{
			Name:   tag,
			UserID: userID,
		}
	})

	var lockingScript []byte
	if output.LockingScript != nil {
		var err error
		lockingScript, err = output.LockingScript.ToBytes()
		if err != nil {
			return nil, fmt.Errorf("failed to convert locking script to bytes: %w", err)
		}
	}

	out := models.Output{
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
		LockingScript:      lockingScript,
		CustomInstructions: output.CustomInstructions,
		SenderIdentityKey:  output.SenderIdentityKey,
		BasketName:         output.BasketName,
		Tags:               tags,
	}

	if out.Spendable && out.Change {
		if is.EmptyString(output.BasketName) {
			return nil, fmt.Errorf("basket not provided for change output")
		}
		if out.Satoshis == 0 {
			return nil, fmt.Errorf("change output with zero satoshis")
		}
		sats, err := to.UInt64(out.Satoshis)
		if err != nil {
			return nil, fmt.Errorf("failed to convert satoshis to uint64: %w", err)
		}

		out.UserUTXO = &models.UserUTXO{
			UserID:             userID,
			Satoshis:           sats,
			EstimatedInputSize: txutils.EstimatedInputSizeByType(output.Type),
			UTXOStatus:         utxoStatus,
		}
	}
	return &out, nil
}

func (txs *Transactions) markReservedOutputsAsNotSpendable(tx *gorm.DB, spendingTransactionID uint, userID int, outputIDs []uint) error {
	if len(outputIDs) == 0 {
		return nil
	}

	err := tx.Model(&models.Output{}).
		Where("id IN ?", outputIDs).
		Where("user_id = ?", userID).
		Updates(map[string]interface{}{
			"spendable": false,
			"spent_by":  spendingTransactionID,
		}).
		Error
	if err != nil {
		return fmt.Errorf("failed to mark reserved outputs as not spendable: %w", err)
	}
	return nil
}

func (txs *Transactions) FindTransactionByUserIDAndTxID(ctx context.Context, userID int, txID string) (*entity.Transaction, error) {
	var transaction models.Transaction
	err := txs.db.WithContext(ctx).Scopes(scopes.UserID(userID)).Where("tx_id = ?", txID).First(&transaction).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find transaction: %w", err)
	}

	return txs.mapModelToTransactionEntity(&transaction), nil
}

func (txs *Transactions) FindTransactionIDsByTxID(ctx context.Context, txID string) ([]uint, error) {
	var transactions []*models.Transaction
	err := txs.db.WithContext(ctx).
		Select(txs.query.Transaction.ID.ColumnName().String()).
		Where(txs.query.Transaction.TxID.Eq(txID)).
		Find(&transactions).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find transaction IDs by TxID: %w", err)
	}

	return slices.Map(transactions, func(tx *models.Transaction) uint {
		return tx.ID
	}), nil
}

func (txs *Transactions) FindTransactionByReference(ctx context.Context, userID int, reference string) (*entity.Transaction, error) {
	var transaction models.Transaction
	err := txs.db.WithContext(ctx).
		Scopes(scopes.UserID(userID)).
		Where("reference = ?", reference).
		Preload("Labels").
		First(&transaction).
		Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, fmt.Errorf("failed to find transaction by reference: %w", err)
	}

	return txs.mapModelToTransactionEntity(&transaction), nil
}

func (txs *Transactions) SpendTransaction(ctx context.Context, updatedTx entity.UpdatedTx, txNote history.Builder) error {
	err := txs.db.WithContext(ctx).Transaction(func(tx *gorm.DB) (err error) {
		err = tx.Model(models.Transaction{}).
			Scopes(scopes.UserID(updatedTx.UserID)).
			Where("id = ?", updatedTx.TransactionID).
			Updates(map[string]any{
				"tx_id":      updatedTx.TxID,
				"input_beef": nil, // input_beef per user's transaction won't be needed anymore; it is moved to the KnownTx (storage-wide)
				"status":     updatedTx.TxStatus,
			}).Error
		if err != nil {
			return err
		}

		err = tx.Delete(models.UserUTXO{}, "reserved_by_id = ?", updatedTx.TransactionID).Error
		if err != nil {
			return err
		}

		var changeOutputs []*models.Output
		err = tx.Model(&models.Output{}).
			Select(txs.query.Output.ID.ColumnName().String(), txs.query.Output.Vout.ColumnName().String()).
			Scopes(scopes.UserID(updatedTx.UserID)).
			Where(txs.query.Output.TransactionID.Eq(updatedTx.TransactionID)).
			Where(txs.query.Output.BasketName.IsNotNull()).
			Where(txs.query.Output.Change.Is(true)).
			Where(txs.query.Output.Satoshis.Gt(0)).
			Where(txs.query.Output.SpentBy.IsNull()).
			Find(&changeOutputs).Error
		if err != nil {
			return fmt.Errorf("failed to find outputs for transaction: %w", err)
		}

		for _, output := range changeOutputs {
			lockingScript, err := updatedTx.GetLockingScriptBytes(output.Vout)
			if err != nil {
				return fmt.Errorf("failed to get locking script: %w", err)
			}

			err = tx.Model(&models.Output{}).
				Where("id = ?", output.ID).
				Updates(map[string]any{
					txs.query.Output.LockingScript.ColumnName().String(): lockingScript,
				}).Error
			if err != nil {
				return fmt.Errorf("failed to update locking script for change output: %w", err)
			}
		}

		return upsertKnownTx(tx, &entity.UpsertKnownTx{
			TxID:          updatedTx.TxID,
			Status:        updatedTx.ReqTxStatus,
			RawTx:         updatedTx.RawTx,
			InputBeef:     updatedTx.InputBeef,
			SkipForStatus: to.Ptr(wdk.ProvenTxStatusCompleted),
		}, txNote)
	})
	if err != nil {
		return fmt.Errorf("failed to update transaction: %w", err)
	}
	return nil
}

func (txs *Transactions) UpdateTransactionStatusByTxID(ctx context.Context, txID string, txStatus wdk.TxStatus) error {
	return txs.db.WithContext(ctx).Model(models.Transaction{}).
		Where("tx_id = ?", txID).
		Updates(map[string]any{
			"status": txStatus,
		}).Error
}

func (txs *Transactions) checkIfAnyOutputIsSpent(tx *gorm.DB, transactionID uint) error {
	var spentCount int64
	err := tx.Model(&models.Output{}).
		Where("transaction_id = ?", transactionID).
		Where("spent_by IS NOT NULL").
		Count(&spentCount).Error
	if err != nil {
		return fmt.Errorf("failed to count spent outputs: %w", err)
	}

	if spentCount > 0 {
		return fmt.Errorf("transaction with ID %d has spent outputs", transactionID)
	}

	return nil
}

func (txs *Transactions) updateTransactionStatusByID(tx *gorm.DB, transactionID uint, newStatus wdk.TxStatus) error {
	return tx.Model(&models.Transaction{}).
		Where("id = ?", transactionID).
		Updates(map[string]any{
			"status": newStatus,
		}).Error
}

func (txs *Transactions) deleteOutputsByTransactionID(tx *gorm.DB, transactionID uint) error {
	return tx.Delete(&models.Output{}, "transaction_id = ?", transactionID).Error
}

func (txs *Transactions) releaseOutputsReservedByTransaction(tx *gorm.DB, transactionID uint) error {
	return tx.Model(&models.Output{}).
		Where("spent_by = ?", transactionID).
		Updates(map[string]any{
			"spent_by":  nil,
			"spendable": true,
		}).Error
}

func (txs *Transactions) releaseReservedUTXOs(tx *gorm.DB, transactionID uint) error {
	return tx.Model(&models.UserUTXO{}).
		Where("reserved_by_id = ?", transactionID).
		Update("reserved_by_id", nil).Error
}

func (txs *Transactions) mapModelToTransactionEntity(model *models.Transaction) *entity.Transaction {
	return &entity.Transaction{
		ID:          model.ID,
		CreatedAt:   model.CreatedAt,
		UpdatedAt:   model.UpdatedAt,
		UserID:      model.UserID,
		Status:      model.Status,
		Reference:   model.Reference,
		IsOutgoing:  model.IsOutgoing,
		Satoshis:    model.Satoshis,
		Description: model.Description,
		Version:     model.Version,
		LockTime:    model.LockTime,
		TxID:        model.TxID,
		InputBEEF:   model.InputBeef,
		Labels: slices.Map(model.Labels, func(label *models.Label) string {
			return label.Name
		}),
	}
}

func (txs *Transactions) ListAndCountActions(ctx context.Context, userID int, filter entity.ListActionsFilter) ([]*entity.Transaction, int64, error) {
	var (
		total    int64
		entities []*entity.Transaction
	)

	err := txs.db.Transaction(func(tx *gorm.DB) error {
		query := tx.
			WithContext(ctx).
			Model(&models.Transaction{}).
			Debug().
			Where("user_id = ?", userID)

		if len(filter.Status) > 0 {
			query = query.Where("status IN ?", filter.Status)
		}
		if len(filter.Labels) > 0 {
			query = query.Scopes(txs.labelFilterScope(tx, userID, filter))
		}

		if err := query.Count(&total).Error; err != nil {
			return fmt.Errorf("count failed: %w", err)
		}

		if total == 0 {
			return nil
		}

		knownItemsCount := int(total)

		if filter.Offset > 0 {
			query = query.Offset(filter.Offset)
		}
		if filter.Limit > 0 {
			query = query.Limit(filter.Limit)
			if filter.Limit < knownItemsCount {
				knownItemsCount = filter.Limit
			}
		}

		rows, err := query.Order("id ASC").Rows()
		if err != nil {
			return fmt.Errorf("query rows failed: %w", err)
		}
		defer rows.Close()

		entities, err = seqerr.ToSlice(
			seqerr.Map(
				batchedRowsIter[models.Transaction](txs.db, rows),
				txs.mapModelToTransactionEntity,
			),
			make([]*entity.Transaction, 0, knownItemsCount),
		)
		if err != nil {
			return fmt.Errorf("failed to read rows: %w", err)
		}

		if err := rows.Err(); err != nil {
			return fmt.Errorf("row iteration failed: %w", err)
		}
		return nil
	})

	if err != nil {
		return nil, 0, fmt.Errorf("failed to run transaction query: %w", err)
	}

	return entities, total, nil
}

func batchedRowsIter[T any](db *gorm.DB, rows *sql.Rows) iter.Seq2[*T, error] {
	return func(yield func(*T, error) bool) {
		for rows.Next() {
			var model T
			if err := db.ScanRows(rows, &model); err != nil {
				yield(nil, fmt.Errorf("scan failed: %w", err))
				return
			}

			if !yield(&model, nil) {
				return
			}
		}
	}
}

func (txs *Transactions) GetLabelsForTransactions(ctx context.Context, txIDs []uint) (map[uint][]string, error) {
	if len(txIDs) == 0 {
		return make(map[uint][]string), nil
	}

	type resultRow struct {
		TransactionID uint
		LabelName     string
	}

	var rows []resultRow
	err := txs.db.WithContext(ctx).
		Model(&models.TransactionLabel{}).
		Select("transaction_id, label_name").
		Where("transaction_id IN ?", txIDs).
		Where("label_name IS NOT NULL").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("failed to batch fetch labels: %w", err)
	}

	labelsMap := make(map[uint][]string)
	for _, row := range rows {
		labelsMap[row.TransactionID] = append(labelsMap[row.TransactionID], row.LabelName)
	}
	return labelsMap, nil
}

func (txs *Transactions) AddLabels(ctx context.Context, userID int, transactionID uint, labels ...string) error {
	newLabels := slices.Map(labels, func(value string) any {
		return &models.Label{
			Name:   value,
			UserID: userID,
		}
	})

	transactionModel := models.Transaction{}

	err := txs.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Model(models.Transaction{}).
			Select("*").
			Where("id = ?", transactionID).
			Preload("Labels").
			First(&transactionModel).Error
		if err != nil {
			return fmt.Errorf("failed to find transaction: %w", err)
		}

		association := tx.
			Model(&transactionModel).
			Association("Labels")

		err = association.Append(newLabels...)
		if err != nil {
			return fmt.Errorf("failed to append new labels: %w", err)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to replace labels: %w", err)
	}

	return nil
}

func (txs *Transactions) labelFilterScope(tx *gorm.DB, userID int, filter entity.ListActionsFilter) func(db *gorm.DB) *gorm.DB {
	return func(query *gorm.DB) *gorm.DB {
		subQuery := tx.Model(&models.TransactionLabel{}).
			Select("transaction_id").
			Where("label_name IN ?", filter.Labels).
			Where("label_user_id = ?", userID)

		if filter.LabelQueryMode == defs.QueryModeAll {
			subQuery = subQuery.Group("transaction_id").Having("COUNT(DISTINCT label_name) = ?", len(filter.Labels))
		}

		return query.Where("id IN (?)", subQuery)
	}
}

func (txs *Transactions) AbortTransactionAtomic(ctx context.Context, transactionID uint, txID *string, reference string) error {
	if err := txs.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		historyBuilders := []history.Builder{history.NewBuilder().AbortAction(reference)}

		for _, step := range abortTransactionSteps {
			if err := step.fn(txs, tx, transactionID); err != nil {
				return fmt.Errorf("AbortTransactionAtomic: step '%s' failed: %w", step.name, err)
			}
		}

		if txID == nil || *txID == "" {
			return nil
		}

		if err := updateKnownTxStatus(tx, *txID, wdk.ProvenTxStatusInvalid, nil, historyBuilders); err != nil {
			return fmt.Errorf("AbortTransactionAtomic: updateKnownTxStatus failed: %w", err)

		}

		return nil
	}); err != nil {
		return fmt.Errorf("failed to abort transaction: %w", err)
	}

	return nil
}
