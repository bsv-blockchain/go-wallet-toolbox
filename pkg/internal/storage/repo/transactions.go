package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/go-softwarelab/common/pkg/slices"
	"github.com/go-softwarelab/common/pkg/to"
	"go.opentelemetry.io/otel/attribute"
	"gorm.io/gen"
	"gorm.io/gen/field"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	pkgentity "github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/genquery"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/scopes"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/history"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/tracing"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

// ErrUTXOContention is returned when concurrent transactions attempt to reserve the same UTXOs.
// The caller should retry the operation.
var ErrUTXOContention = errors.New("utxo contention: concurrent transaction already reserved one or more of the selected UTXOs")

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

	err = txs.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return txs.createTransactionInTx(tx, newTx)
	})
	if err != nil {
		return fmt.Errorf("failed to create transaction: %w", err)
	}
	return nil
}

// CreateTransactionInTx creates a new transaction record using an externally-managed DB transaction.
// Use this when Fund and CreateTransaction must share a single DB transaction (e.g. SELECT FOR UPDATE).
func (txs *Transactions) CreateTransactionInTx(ctx context.Context, tx *gorm.DB, newTx *entity.NewTx) error {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Transaction-CreateTransactionInTx")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	err = txs.createTransactionInTx(tx.WithContext(ctx), newTx)
	if err != nil {
		return fmt.Errorf("failed to create transaction in tx: %w", err)
	}
	return nil
}

func (txs *Transactions) createTransactionInTx(tx *gorm.DB, newTx *entity.NewTx) error {
	model, err := txs.toTransactionModel(newTx)
	if err != nil {
		return err
	}

	if err = txs.connectOutputsWithBaskets(tx, newTx, model); err != nil {
		return fmt.Errorf("failed to connect outputs with baskets: %w", err)
	}

	if err = tx.Create(model).Error; err != nil {
		return fmt.Errorf("failed to create new transaction model: %w", err)
	}

	if err = linkTxLabels(tx, newTx.UserID, model.TransactionID, model.Labels); err != nil {
		return fmt.Errorf("failed to link transaction labels: %w", err)
	}

	for _, output := range model.Outputs {
		if err = linkOutputTags(tx, newTx.UserID, output.OutputID, output.Tags); err != nil {
			return fmt.Errorf("failed to link output tags: %w", err)
		}
	}

	if err = txs.markReservedOutputsAsNotSpendable(tx, model.TransactionID, newTx.UserID, newTx.SpentOutputIDs); err != nil {
		return fmt.Errorf("failed to mark reserved outputs as not spendable: %w", err)
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
		Version:     ptrUint32(newTx.Version),
		LockTime:    ptrUint32(newTx.LockTime),
		InputBeef:   newTx.InputBeef,
		TxID:        newTx.TxID,
		Labels: slices.Map(newTx.Labels, func(label primitives.StringUnder300) *models.TxLabel {
			return &models.TxLabel{
				Label:  string(label),
				UserID: newTx.UserID,
			}
		}),
		Outputs: outputs,
		Commission: to.If(newTx.Commission != nil, func() *models.Commission {
			return &models.Commission{
				UserID:        newTx.UserID,
				Satoshis:      int(newTx.Commission.Satoshis),
				KeyOffset:     newTx.Commission.KeyOffset,
				IsRedeemed:    newTx.Commission.IsRedeemed,
				LockingScript: newTx.Commission.LockingScript,
			}
		}).ElseThen(nil),
	}

	return model, nil
}

func (txs *Transactions) connectOutputsWithBaskets(tx *gorm.DB, newTx *entity.NewTx, model *models.Transaction) error {
	// TODO: Fix basket association if needed
	return nil
}

func (txs *Transactions) makeNewOutput(userID int, output *entity.NewOutput, utxoStatus wdk.UTXOStatus) (*models.Output, error) {
	tags := slices.Map(output.Tags, func(tag string) *models.OutputTag {
		return &models.OutputTag{
			Tag:    tag,
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
		BasketID:           output.BasketID,

		Tags: tags,
	}
	return &out, nil
}

func (txs *Transactions) markReservedOutputsAsNotSpendable(tx *gorm.DB, spendingTransactionID uint, userID int, outputIDs []uint) error {
	if len(outputIDs) == 0 {
		return nil
	}

	err := tx.Model(&models.Output{}).
		Where("outputId IN ?", outputIDs).
		Where("userId = ?", userID).
		Where("spentBy IS NULL").
		Updates(map[string]interface{}{
			"spendable": false,
			"spentBy":   spendingTransactionID,
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
	err = txs.db.WithContext(ctx).Scopes(scopes.UserID(userID)).Where("txid = ?", txID).First(&transaction).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			err = nil
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find transaction: %w", err)
	}

	transaction.Labels, err = fetchLabelsForTransaction(ctx, txs.db, transaction.TransactionID)
	if err != nil {
		return nil, err
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
		Select(txs.query.Transaction.TransactionID.ColumnName().String()).
		Where(txs.query.Transaction.TxID.Eq(txID)).
		Find(&transactions).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find transaction IDs by TxID: %w", err)
	}

	return slices.Map(transactions, func(tx *models.Transaction) uint {
		return tx.TransactionID
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
		Select("txid", "reference").
		Where("txid IN ?", txIDs).
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
		First(&transaction).
		Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			err = nil
			return nil, nil
		}

		return nil, fmt.Errorf("failed to find transaction by reference: %w", err)
	}

	transaction.Labels, err = fetchLabelsForTransaction(ctx, txs.db, transaction.TransactionID)
	if err != nil {
		return nil, err
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
			Where("transactionId = ?", updatedTx.TransactionID).
			Updates(map[string]any{
				"txid":      updatedTx.TxID,
				"inputBeef": nil, // input_beef per user's transaction won't be needed anymore; it is moved to the KnownTx (storage-wide)
				"status":    updatedTx.TxStatus,
			}).Error
		if err != nil {
			return err
		}

		var changeOutputs []*models.Output
		err = tx.Model(&models.Output{}).
			Select(
				txs.query.Output.OutputID.ColumnName().String(),
				txs.query.Output.TransactionID.ColumnName().String(),
				txs.query.Output.Vout.ColumnName().String(),
			).
			Scopes(scopes.UserID(updatedTx.UserID)).
			Where(txs.query.Output.TransactionID.Eq(updatedTx.TransactionID)).
			Where(txs.query.Output.BasketID.IsNotNull()).
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
				Where("outputId = ?", output.OutputID).
				Updates(map[string]any{
					txs.query.Output.LockingScript.ColumnName().String(): lockingScript,
				}).Error
			if err != nil {
				return fmt.Errorf("failed to update locking script for change output: %w", err)
			}
		}

		return upsertProvenTxReq(tx, &entity.UpsertProvenTxReq{
			TxID:            updatedTx.TxID,
			Status:          updatedTx.ReqTxStatus,
			RawTx:           updatedTx.RawTx,
			InputBeef:       updatedTx.InputBeef,
			SkipForStatuses: []wdk.ProvenTxReqStatus{wdk.ProvenTxStatusCompleted},
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
		Where("txid = ?", txID).
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
		Where(table.TransactionID.Eq(transactionID)).
		Update(table.Status, txStatus)
	if err != nil {
		return fmt.Errorf("update query for transaction status failed: %w", err)
	}
	return nil
}

func (txs *Transactions) mapModelToTransactionEntity(model *models.Transaction) *pkgentity.Transaction {
	return &pkgentity.Transaction{
		ID:          model.TransactionID,
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
		Labels: slices.Map(model.Labels, func(label *models.TxLabel) string {
			return label.Label
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
			Where("userId = ?", userID)

		if len(filter.Status) > 0 {
			query = query.Where("status IN ?", filter.Status)
		}

		if len(filter.Labels) > 0 {
			query = query.Scopes(txs.labelFilterScope(tx, userID, filter))
		}

		if err = query.Count(&total).Error; err != nil {
			return fmt.Errorf("count failed: %w", err)
		}

		if total == 0 {
			return nil
		}

		if err = query.
			Limit(filter.Limit).
			Offset(filter.Offset).
			Order("transactionId ASC").
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

// buildSelectedActionsSubQuery constructs a subquery selecting the current page of actions (transactionId, txid)
// matching the provided filter. It mirrors ListAndCountActions ordering and pagination so it can be
// reused in JOINs to avoid large IN (...) clauses.
func (txs *Transactions) buildSelectedActionsSubQuery(tx *gorm.DB, userID int, filter entity.ListActionsFilter) *gorm.DB {
	query := tx.Model(&models.Transaction{}).
		Select("transactionId, txid").
		Where("userId = ?", userID)

	if len(filter.Status) > 0 {
		query = query.Where("status IN ?", filter.Status)
	}
	if len(filter.Labels) > 0 {
		query = query.Scopes(txs.labelFilterScope(tx, userID, filter))
	}

	return query.Order("transactionId ASC").Limit(filter.Limit).Offset(filter.Offset)
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
		var rows *sql.Rows
		rows, err = tx.Table("bsv_tx_labels tl").
			Select("tlm.transactionId, tl.label AS label_name").
			Joins("JOIN bsv_tx_labels_map tlm ON tlm.txLabelId = tl.txLabelId").
			Joins("JOIN (?) s ON s.transactionId = tlm.transactionId", selected).
			Where("tl.label IS NOT NULL").
			Where("tl.isDeleted = ?", false).
			Where("tlm.isDeleted = ?", false).
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
			if err = rows.Scan(&txID, &label); err != nil {
				return fmt.Errorf("scan failed: %w", err)
			}
			labelsMap[txID] = append(labelsMap[txID], label)
		}
		if err = rows.Err(); err != nil {
			return fmt.Errorf("rows iteration failed: %w", err)
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
		Table("bsv_tx_labels_map tlm").
		Select("tlm.transactionId AS transaction_id, tl.label AS label_name").
		Joins("JOIN bsv_tx_labels tl ON tl.txLabelId = tlm.txLabelId").
		Where("tlm.transactionId IN ?", txIDs).
		Where("tl.isDeleted = ?", false).
		Where("tlm.isDeleted = ?", false).
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

func (txs *Transactions) GetLabelsForTxIDs(ctx context.Context, txIDs []string) (map[string][]string, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Transaction-GetLabelsForTxIDs")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	if len(txIDs) == 0 {
		return make(map[string][]string), nil
	}

	type resultRow struct {
		TxID      string
		LabelName string
	}

	var rows []resultRow
	err = txs.db.WithContext(ctx).
		Table("bsv_transactions t").
		Select("t.txid, tl.label AS label_name").
		Joins("JOIN bsv_tx_labels_map tlm ON tlm.transactionId = t.transactionId").
		Joins("JOIN bsv_tx_labels tl ON tl.txLabelId = tlm.txLabelId").
		Where("t.txid IN ?", txIDs).
		Where("tl.label IS NOT NULL").
		Where("tl.isDeleted = ?", false).
		Where("tlm.isDeleted = ?", false).
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("failed to batch fetch labels by txID: %w", err)
	}

	labelsMap := make(map[string][]string)
	for _, row := range rows {
		labelsMap[row.TxID] = append(labelsMap[row.TxID], row.LabelName)
	}
	return labelsMap, nil
}

func (txs *Transactions) AddLabels(ctx context.Context, userID int, transactionID uint, labels ...string) error {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Transaction-AddLabels", attribute.Int("UserID", userID), attribute.String("TransactionID", fmt.Sprintf("%d", transactionID)), attribute.StringSlice("Labels", labels))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	newLabels := slices.Map(labels, func(value string) *models.TxLabel {
		return &models.TxLabel{
			Label:  value,
			UserID: userID,
		}
	})

	err = txs.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return linkTxLabels(tx, userID, transactionID, newLabels)
	})
	if err != nil {
		return fmt.Errorf("failed to add labels: %w", err)
	}

	return nil
}

func (txs *Transactions) labelFilterScope(tx *gorm.DB, userID int, filter entity.ListActionsFilter) func(db *gorm.DB) *gorm.DB {
	return func(query *gorm.DB) *gorm.DB {
		subQuery := tx.Table("bsv_tx_labels_map tlm").
			Select("tlm.transactionId").
			Joins("JOIN bsv_tx_labels tl ON tl.txLabelId = tlm.txLabelId").
			Where("tl.label IN ?", filter.Labels).
			Where("tl.userId = ?", userID).
			Where("tl.isDeleted = ?", false).
			Where("tlm.isDeleted = ?", false)

		if filter.LabelQueryMode == defs.QueryModeAll {
			subQuery = subQuery.Group("tlm.transactionId").Having("COUNT(DISTINCT tl.label) = ?", len(filter.Labels))
		}

		return query.Where("bsv_transactions.transactionId IN (?)", subQuery)
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
		Select(table.TransactionID).
		Scopes(scopes.FromQueryOptsForGen(table, opts)...).
		Where(table.Status.In(slices.Map(txStatus, func(txStatus wdk.TxStatus) string { return string(txStatus) })...)).
		Find()
	if err != nil {
		return nil, fmt.Errorf("query for finding transaction ids by statuses failed: %w", err)
	}

	return slices.Map(rows, func(row *models.Transaction) uint {
		return row.TransactionID
	}), nil
}

func (txs *Transactions) FindTransactionIDsForAbort(ctx context.Context, opts ...queryopts.Options) ([]uint, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Transaction-FindTransactionIDsForAbort")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	txTable := txs.query.Transaction.TableName()
	knownTxTable := txs.query.ProvenTxReq.TableName()

	query := txs.db.WithContext(ctx).
		Model(&models.Transaction{}).
		Select(txTable+"."+txs.query.Transaction.TransactionID.ColumnName().String()).
		Joins(fmt.Sprintf("LEFT JOIN %s ON %s.txid = %s.txid", knownTxTable, knownTxTable, txTable)).
		Where(
			fmt.Sprintf(
				"(%s.status = ? OR (%s.status = ? AND COALESCE(%s.status, ?) = ?))",
				txTable, txTable, knownTxTable,
			),
			wdk.TxStatusUnsigned,
			wdk.TxStatusUnprocessed,
			string(wdk.ProvenTxStatusUnprocessed),
			wdk.ProvenTxStatusUnprocessed,
		).
		Order(txTable + ".created_at ASC")

	options := queryopts.MergeOptions(opts)
	if options.Until != nil {
		options.Until.ApplyDefaults()
		query = query.Where(fmt.Sprintf("%s.created_at <= ?", txTable), options.Until.Time)
	}
	if options.Page != nil {
		options.Page.ApplyDefaults()
		query = query.Offset(options.Page.Offset).Limit(options.Page.Limit)
	}

	var rows []*models.Transaction
	err = query.Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("query for finding transaction ids for abort failed: %w", err)
	}

	return slices.Map(rows, func(row *models.Transaction) uint {
		return row.TransactionID
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
		Version:     derefUint32(tx.Version),
		LockTime:    derefUint32(tx.LockTime),
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

	_, err = table.WithContext(ctx).Where(table.TransactionID.Eq(spec.ID)).Updates(updates)
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
		Find()
	if err != nil {
		return nil, fmt.Errorf("failed to find transactions: %w", err)
	}

	for _, row := range rows {
		row.Labels, err = fetchLabelsForTransaction(ctx, txs.db, row.TransactionID)
		if err != nil {
			return nil, err
		}
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
		return []gen.Condition{table.TransactionID.Eq(*spec.ID)}
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
	if spec.ID != nil {
		conditions = append(conditions, table.TransactionID.Eq(*spec.ID))
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
	tl := &txs.query.TxLabel
	txl := &txs.query.TxLabelsMap

	if labels.Empty {
		sub := txl.WithContext(ctx).
			Select(txl.TransactionID).
			Where(txl.TransactionID.EqCol(table.TransactionID))

		return []gen.Condition{
			field.Not(field.CompareSubQuery(field.ExistsOp, nil, sub.UnderlyingDB())),
		}
	}

	if len(labels.ContainAny) > 0 {
		sub := txl.WithContext(ctx).
			Join(tl, tl.TxLabelID.EqCol(txl.TxLabelID)).
			Select(txl.TransactionID).
			Where(
				tl.Label.In(labels.ContainAny...),
				txl.TransactionID.EqCol(table.TransactionID),
			)
		conds = append(conds, gen.Exists(sub))
	}

	if len(labels.ContainAll) > 0 {
		for _, label := range labels.ContainAll {
			sub := txl.WithContext(ctx).
				Join(tl, tl.TxLabelID.EqCol(txl.TxLabelID)).
				Select(txl.TransactionID).
				Where(
					tl.Label.Eq(label),
					txl.TransactionID.EqCol(table.TransactionID),
				)
			conds = append(conds, gen.Exists(sub))
		}
	}

	return conds
}

// linkTxLabels upserts each label (by label+userId) and links it to transactionID via
// bsv_tx_labels_map, leaving any labels the transaction already had (not present in `labels`) untouched.
func linkTxLabels(tx *gorm.DB, userID int, transactionID uint, labels []*models.TxLabel) error {
	for _, l := range labels {
		labelID, err := upsertTxLabel(tx, userID, l.Label)
		if err != nil {
			return err
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).
			Create(&models.TxLabelsMap{TxLabelID: labelID, TransactionID: transactionID}).Error; err != nil {
			return fmt.Errorf("failed to link tx label %q to transaction %d: %w", l.Label, transactionID, err)
		}
	}
	return nil
}

func upsertTxLabel(tx *gorm.DB, userID int, label string) (uint, error) {
	m := &models.TxLabel{Label: label, UserID: userID}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(m).Error; err != nil {
		return 0, fmt.Errorf("failed to upsert tx label %q: %w", label, err)
	}
	if m.TxLabelID != 0 {
		return m.TxLabelID, nil
	}

	var existing models.TxLabel
	if err := tx.Where("label = ? AND userId = ?", label, userID).First(&existing).Error; err != nil {
		return 0, fmt.Errorf("failed to find existing tx label %q: %w", label, err)
	}
	return existing.TxLabelID, nil
}

// fetchLabelsForTransaction loads the labels linked to transactionID via bsv_tx_labels_map.
func fetchLabelsForTransaction(ctx context.Context, tx *gorm.DB, transactionID uint) ([]*models.TxLabel, error) {
	var labels []*models.TxLabel
	err := tx.WithContext(ctx).
		Table("bsv_tx_labels tl").
		Select("tl.*").
		Joins("JOIN bsv_tx_labels_map tlm ON tlm.txLabelId = tl.txLabelId").
		Where("tlm.transactionId = ?", transactionID).
		Where("tl.isDeleted = ?", false).
		Where("tlm.isDeleted = ?", false).
		Scan(&labels).Error
	if err != nil {
		return nil, fmt.Errorf("failed to fetch labels for transaction: %w", err)
	}
	return labels, nil
}

func derefUint32(v *uint32) uint32 {
	if v == nil {
		return 0
	}
	return *v
}

func ptrUint32(v uint32) *uint32 {
	return &v
}
