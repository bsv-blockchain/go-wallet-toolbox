package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	pkgentity "github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/genquery"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/scopes"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/history"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/txutils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/tracing"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/go-softwarelab/common/pkg/is"
	"github.com/go-softwarelab/common/pkg/slices"
	"github.com/go-softwarelab/common/pkg/to"
	"go.opentelemetry.io/otel/attribute"
	"gorm.io/gen"
	"gorm.io/gen/field"
	"gorm.io/gorm"
)

type Transactions struct {
	query *genquery.Query
	db    *gorm.DB
}

func NewTransactions(db *gorm.DB, query *genquery.Query) *Transactions {
	return &Transactions{db: db, query: query}
}

func (txs *Transactions) CreateTransaction(ctx context.Context, newTx *entity.NewTx) error {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Transaction-CreateTransaction")
	defer func() {
		tracing.EndTracing(span, err)
	}()

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

func (txs *Transactions) FindTransactionByUserIDAndTxID(ctx context.Context, userID int, txID string) (*pkgentity.Transaction, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Transaction-FindTransactionByUserIDAndTxID", attribute.Int("UserID", userID), attribute.String("TxID", txID))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	var transaction models.Transaction
	err = txs.db.WithContext(ctx).Scopes(scopes.UserID(userID)).Where("tx_id = ?", txID).First(&transaction).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			err = nil
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find transaction: %w", err)
	}

	return txs.mapModelToTransactionEntity(&transaction), nil
}

func (txs *Transactions) FindTransactionIDsByTxID(ctx context.Context, txID string) ([]uint, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Transaction-FindTransactionIDsByTxID", attribute.String("TxID", txID))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	var transactions []*models.Transaction
	err = txs.db.WithContext(ctx).
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

func (txs *Transactions) FindReferencesByTxIDs(ctx context.Context, txIDs []string) (map[string]string, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Transaction-FindReferencesByTxIDs")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	if len(txIDs) == 0 {
		return make(map[string]string), nil
	}

	var transactions []*models.Transaction
	err = txs.db.WithContext(ctx).
		Select("tx_id", "reference").
		Where("tx_id IN ?", txIDs).
		Find(&transactions).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find references by TxIDs: %w", err)
	}

	result := make(map[string]string, len(transactions))
	for _, tx := range transactions {
		if tx.TxID != nil {
			result[*tx.TxID] = tx.Reference
		}
	}

	return result, nil
}

func (txs *Transactions) FindTransactionByReference(ctx context.Context, userID int, reference string) (*pkgentity.Transaction, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Transaction-FindTransactionByReference", attribute.Int("UserID", userID), attribute.String("Reference", reference))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	var transaction models.Transaction
	err = txs.db.WithContext(ctx).
		Scopes(scopes.UserID(userID)).
		Where("reference = ?", reference).
		Preload("Labels").
		First(&transaction).
		Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			err = nil
			return nil, nil
		}

		return nil, fmt.Errorf("failed to find transaction by reference: %w", err)
	}

	return txs.mapModelToTransactionEntity(&transaction), nil
}

func (txs *Transactions) SpendTransaction(ctx context.Context, updatedTx entity.UpdatedTx, txNote history.Builder) error {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Transaction-SpendTransaction", attribute.Int("UserID", updatedTx.UserID), attribute.String("TransactionID", updatedTx.TxID))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	err = txs.db.WithContext(ctx).Transaction(func(tx *gorm.DB) (err error) {
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
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Transaction-UpdateTransactionStatusByTxID", attribute.String("TransactionID", txID), attribute.String("Status", string(txStatus)))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	err = txs.db.WithContext(ctx).Model(models.Transaction{}).
		Where("tx_id = ?", txID).
		Updates(map[string]any{
			"status": txStatus,
		}).Error

	if err != nil {
		return fmt.Errorf("failed to update transaction status by txID: %w", err)
	}

	return nil
}

func (txs *Transactions) UpdateTransactionStatusByID(ctx context.Context, transactionID uint, txStatus wdk.TxStatus) error {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Transaction-UpdateTransactionStatusByID", attribute.String("TransactionID", fmt.Sprintf("%d", transactionID)), attribute.String("Status", string(txStatus)))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	table := txs.query.Transaction
	_, err = table.WithContext(ctx).
		Where(table.ID.Eq(transactionID)).
		Update(table.Status, txStatus)

	if err != nil {
		return fmt.Errorf("update query for transaction status failed: %w", err)
	}
	return nil
}

func (txs *Transactions) mapModelToTransactionEntity(model *models.Transaction) *pkgentity.Transaction {
	return &pkgentity.Transaction{
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

func (txs *Transactions) ListAndCountActions(ctx context.Context, userID int, filter entity.ListActionsFilter) ([]*pkgentity.Transaction, int64, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Transaction-ListAndCountActions", attribute.Int("UserID", userID))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	var actions []*models.Transaction
	var total int64

	err = txs.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Model(&models.Transaction{}).
			Where("user_id = ?", userID)

		if len(filter.Status) > 0 {
			query = query.Where("status IN ?", filter.Status)
		}

		if len(filter.Labels) > 0 {
			query = query.Scopes(txs.labelFilterScope(tx, userID, filter))
		}

		if filter.Reference != nil && *filter.Reference != "" {
			query = query.Where("reference = ?", *filter.Reference)
		}

		if err := query.Count(&total).Error; err != nil {
			return fmt.Errorf("count failed: %w", err)
		}

		if total == 0 {
			return nil
		}

		if err := query.
			Limit(filter.Limit).
			Offset(filter.Offset).
			Order("id ASC").
			Find(&actions).Error; err != nil {
			return fmt.Errorf("query failed: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, 0, fmt.Errorf("transaction failed: %w", err)
	}

	return slices.Map(actions, txs.mapModelToTransactionEntity), total, nil
}

// buildSelectedActionsSubQuery constructs a subquery selecting the current page of actions (id, tx_id)
// matching the provided filter. It mirrors ListAndCountActions ordering and pagination so it can be
// reused in JOINs to avoid large IN (...) clauses.
func (txs *Transactions) buildSelectedActionsSubQuery(tx *gorm.DB, userID int, filter entity.ListActionsFilter) *gorm.DB {
	query := tx.Model(&models.Transaction{}).
		Select("id, tx_id").
		Where("user_id = ?", userID)

	if len(filter.Status) > 0 {
		query = query.Where("status IN ?", filter.Status)
	}
	if len(filter.Labels) > 0 {
		query = query.Scopes(txs.labelFilterScope(tx, userID, filter))
	}
	if filter.Reference != nil && *filter.Reference != "" {
		query = query.Where("reference = ?", *filter.Reference)
	}

	return query.Order("id ASC").Limit(filter.Limit).Offset(filter.Offset)
}

// GetLabelsForSelectedActions fetches labels via JOIN with the selected actions subquery to avoid IN lists.
func (txs *Transactions) GetLabelsForSelectedActions(ctx context.Context, userID int, filter entity.ListActionsFilter) (map[uint][]string, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Transaction-GetLabelsForSelectedActions", attribute.Int("UserID", userID))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	labelsMap := make(map[uint][]string)
	err = txs.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		selected := txs.buildSelectedActionsSubQuery(tx, userID, filter)
		var closeErr error
		rows, err := tx.Table("bsv_transaction_labels tl").
			Select("tl.transaction_id, tl.label_name").
			Joins("JOIN (?) s ON s.id = tl.transaction_id", selected).
			Where("tl.label_name IS NOT NULL").
			Where("tl.deleted_at IS NULL").
			Rows()
		if err != nil {
			return fmt.Errorf("failed to query labels rows: %w", err)
		}
		defer func() {
			if cerr := rows.Close(); cerr != nil {
				closeErr = fmt.Errorf("rows close failed: %w", cerr)
			}
		}()

		for rows.Next() {
			var txID uint
			var label string
			if err := rows.Scan(&txID, &label); err != nil {
				return fmt.Errorf("scan failed: %w", err)
			}
			labelsMap[txID] = append(labelsMap[txID], label)
		}
		return closeErr
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch labels for selected actions: %w", err)
	}
	return labelsMap, nil
}

func (txs *Transactions) GetLabelsForTransactions(ctx context.Context, txIDs []uint) (map[uint][]string, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Transaction-GetLabelsForTransactions")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	if len(txIDs) == 0 {
		return make(map[uint][]string), nil
	}

	type resultRow struct {
		TransactionID uint
		LabelName     string
	}

	var rows []resultRow
	err = txs.db.WithContext(ctx).
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
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Transaction-AddLabels", attribute.Int("UserID", userID), attribute.String("TransactionID", fmt.Sprintf("%d", transactionID)), attribute.StringSlice("Labels", labels))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	newLabels := slices.Map(labels, func(value string) any {
		return &models.Label{
			Name:   value,
			UserID: userID,
		}
	})

	transactionModel := models.Transaction{}

	err = txs.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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

func (txs *Transactions) FindTransactionIDsByStatuses(ctx context.Context, txStatus []wdk.TxStatus, opts ...queryopts.Options) ([]uint, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Transaction-FindTransactionIDsByStatuses")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	table := &txs.query.Transaction
	rows, err := table.WithContext(ctx).
		Select(table.ID).
		Scopes(scopes.FromQueryOptsForGen(table, opts)...).
		Where(table.Status.In(slices.Map(txStatus, func(txStatus wdk.TxStatus) string { return string(txStatus) })...)).
		Find()
	if err != nil {
		return nil, fmt.Errorf("query for finding transaction ids by statuses failed: %w", err)
	}

	return slices.Map(rows, func(row *models.Transaction) uint {
		return row.ID
	}), nil
}

func (txs *Transactions) AddTransaction(ctx context.Context, tx *pkgentity.Transaction) error {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Transaction-AddTransaction")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	if tx == nil {
		return fmt.Errorf("transaction cannot be nil")
	}

	labels := make([]primitives.StringUnder300, len(tx.Labels))
	for i, label := range tx.Labels {
		labels[i] = primitives.StringUnder300(label)
	}

	newTx := &entity.NewTx{
		UserID:      tx.UserID,
		Status:      tx.Status,
		Reference:   tx.Reference,
		IsOutgoing:  tx.IsOutgoing,
		Satoshis:    tx.Satoshis,
		Description: tx.Description,
		Version:     tx.Version,
		LockTime:    tx.LockTime,
		TxID:        tx.TxID,
		Labels:      labels,
	}

	return txs.CreateTransaction(ctx, newTx)
}

func (txs *Transactions) UpdateTransaction(ctx context.Context, spec *pkgentity.TransactionUpdateSpecification) error {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Transaction-UpdateTransaction", attribute.String("TransactionID", fmt.Sprintf("%d", spec.ID)))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	table := &txs.query.Transaction

	updates := map[string]any{}
	if spec.Status != nil {
		updates[table.Status.ColumnName().String()] = *spec.Status
	}
	if spec.Description != nil {
		updates[table.Description.ColumnName().String()] = *spec.Description
	}

	if len(updates) == 0 {
		return nil
	}

	_, err = table.WithContext(ctx).Where(table.ID.Eq(spec.ID)).Updates(updates)
	if err != nil {
		return fmt.Errorf("failed to update transaction: %w", err)
	}

	return nil
}

func (txs *Transactions) FindTransactions(ctx context.Context, spec *pkgentity.TransactionReadSpecification, opts ...queryopts.Options) ([]*pkgentity.Transaction, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Transaction-FindTransactions")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	table := &txs.query.Transaction

	rows, err := table.WithContext(ctx).
		Scopes(scopes.FromQueryOptsForGen(table, opts)...).
		Where(txs.conditionsBySpec(ctx, spec)...).
		Preload(table.Labels).
		Find()
	if err != nil {
		return nil, fmt.Errorf("failed to find transactions: %w", err)
	}

	return slices.Map(rows, txs.mapModelToTransactionEntity), nil
}

func (txs *Transactions) CountTransactions(ctx context.Context, spec *pkgentity.TransactionReadSpecification, opts ...queryopts.Options) (int64, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Transaction-CountTransactions")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	table := &txs.query.Transaction

	count, err := table.WithContext(ctx).
		Scopes(scopes.FromQueryOptsForGen(table, opts)...).
		Where(txs.conditionsBySpec(ctx, spec)...).
		Count()
	if err != nil {
		return 0, fmt.Errorf("failed to count transactions: %w", err)
	}

	return count, nil
}

func (txs *Transactions) conditionsBySpec(ctx context.Context, spec *pkgentity.TransactionReadSpecification) []gen.Condition {
	if spec == nil {
		return nil
	}

	table := &txs.query.Transaction
	if spec.ID != nil {
		return []gen.Condition{table.ID.Eq(*spec.ID)}
	}

	var conditions []gen.Condition
	if spec.UserID != nil {
		conditions = append(conditions, cmpCondition(table.UserID, spec.UserID))
	}
	if spec.Status != nil {
		conditions = append(conditions, cmpCondition(table.Status, spec.Status.ToStringComparable()))
	}
	if spec.Reference != nil {
		conditions = append(conditions, cmpCondition(table.Reference, spec.Reference))
	}
	if spec.IsOutgoing != nil {
		conditions = append(conditions, cmpBoolCondition(table.IsOutgoing, spec.IsOutgoing))
	}
	if spec.Satoshis != nil {
		conditions = append(conditions, cmpCondition(table.Satoshis, spec.Satoshis))
	}
	if spec.TxID != nil {
		conditions = append(conditions, cmpCondition(table.TxID, spec.TxID))
	}
	if spec.DescriptionContains != nil {
		conditions = append(conditions, cmpCondition(table.Description, spec.DescriptionContains))
	}
	if spec.Labels != nil {
		conditions = append(conditions, txs.labelConditions(ctx, spec.Labels)...)
	}

	return conditions
}

func (txs *Transactions) labelConditions(ctx context.Context, labels *pkgentity.ComparableSet[string]) []gen.Condition {
	var conds []gen.Condition
	table := &txs.query.Transaction
	txl := &txs.query.TransactionLabel

	if labels.Empty {
		sub := txl.WithContext(ctx).
			Select(txl.TransactionID).
			Where(txl.TransactionID.EqCol(table.ID))

		return []gen.Condition{
			field.Not(field.CompareSubQuery(field.ExistsOp, nil, sub.UnderlyingDB())),
		}
	}

	if len(labels.ContainAny) > 0 {
		sub := txl.WithContext(ctx).
			Select(txl.TransactionID).
			Where(
				txl.LabelName.In(labels.ContainAny...),
				txl.TransactionID.EqCol(table.ID),
			)
		conds = append(conds, gen.Exists(sub))
	}

	if len(labels.ContainAll) > 0 {
		for _, label := range labels.ContainAll {
			sub := txl.WithContext(ctx).
				Select(txl.TransactionID).
				Where(
					txl.LabelName.Eq(label),
					txl.TransactionID.EqCol(table.ID),
				)
			conds = append(conds, gen.Exists(sub))
		}
	}

	return conds
}
