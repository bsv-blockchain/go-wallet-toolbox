package repo

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"gorm.io/gen/field"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/genquery"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/tracing"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type UTXOs struct {
	db    *gorm.DB
	query *genquery.Query
}

func NewUTXOs(db *gorm.DB, query *genquery.Query) *UTXOs {
	return &UTXOs{
		db:    db,
		query: query,
	}
}

// FindNotReservedUTXOsForUpdate selects not-reserved UTXOs within the provided DB transaction.
// On Postgres/MySQL it appends SELECT FOR UPDATE SKIP LOCKED so concurrent callers automatically
// pick non-overlapping UTXO sets. On SQLite the lock is omitted since SQLite serializes writes;
// the guarded UPDATE in CreateTransactionInTx handles contention there instead.
func (u *UTXOs) FindNotReservedUTXOsForUpdate(
	ctx context.Context,
	tx *gorm.DB,
	userID int,
	basketID *uint,
	page *queryopts.Paging,
	forbiddenOutputIDs []uint,
	includeSending bool,
) ([]*models.Output, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Utxos-FindNotReservedUTXOsForUpdate", attribute.Int("UserID", userID), attribute.String("BasketID", fmt.Sprintf("%v", basketID)), attribute.Bool("IncludeSending", includeSending))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	page.ApplyDefaults()

	statuses := []string{string(wdk.TxStatusCompleted), string(wdk.TxStatusUnproven)}
	if includeSending {
		statuses = append(statuses, string(wdk.TxStatusSending))
	}

	// Order by safety tier (mined=0, unproven=1, sending=2) then satoshis ascending
	// so the collector picks the safest, smallest-sufficient UTXOs first.
	orderClause := fmt.Sprintf(
		"CASE bsv_transactions.status WHEN '%s' THEN 0 WHEN '%s' THEN 1 WHEN '%s' THEN 2 END ASC, bsv_outputs.satoshis ASC",
		wdk.TxStatusCompleted, wdk.TxStatusUnproven, wdk.TxStatusSending,
	)

	query := tx.WithContext(ctx).
		Table("bsv_outputs").
		Joins("INNER JOIN bsv_transactions ON bsv_outputs.transactionId = bsv_transactions.transactionId").
		Where("bsv_outputs.spendable = ?", true).
		Where("bsv_outputs.userId = ?", userID).
		Where("bsv_transactions.status IN ?", statuses).
		Where("bsv_outputs.spentBy IS NULL").
		Order(orderClause).
		Offset(page.Offset).
		Limit(page.Limit)

	if basketID != nil {
		query = query.Where("bsv_outputs.basketId = ?", *basketID)
	} else {
		query = query.Where("bsv_outputs.basketId IS NULL")
	}

	if len(forbiddenOutputIDs) > 0 {
		query = query.Where("bsv_outputs.outputId NOT IN ?", forbiddenOutputIDs)
	}

	if tx.Name() != "sqlite" {
		query = query.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"})
	}

	var result []*models.Output
	err = query.Find(&result).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find and lock not reserved UTXOs: %w", err)
	}
	return result, nil
}

func (u *UTXOs) CountUTXOs(ctx context.Context, userID int, basketID *uint) (int64, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Utxos-CountUTXOs", attribute.Int("UserID", userID), attribute.String("BasketID", fmt.Sprintf("%v", basketID)))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	count := int64(0)

	query := u.db.WithContext(ctx).
		Model(&models.Output{}).
		Where("spendable = ?", true).
		Where("userId = ?", userID).
		Where("spentBy IS NULL")

	if basketID != nil {
		query = query.Where("basketId = ?", *basketID)
	} else {
		query = query.Where("basketId IS NULL")
	}

	err = query.Count(&count).Error

	return count, err
}

func (u *UTXOs) UnreserveUTXOsByTransactionID(ctx context.Context, transactionID uint) error {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Utxos-UnreserveUTXOsByTransactionID", attribute.String("TransactionID", fmt.Sprintf("%d", transactionID)))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	table := u.query.Output
	_, err = table.WithContext(ctx).
		Where(table.SpentBy.Eq(transactionID)).
		Update(table.SpentBy, nil)
	if err != nil {
		return fmt.Errorf("failed to unreserve UTXOs by transaction ID %d: %w", transactionID, err)
	}

	return nil
}

func (u *UTXOs) CreateUTXOForSpendableOutputsByTxID(ctx context.Context, txID string) error {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Utxos-CreateUTXOForSpendableOutputsByTxID", attribute.String("TxID", txID))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	txTable := &u.query.Transaction
	outTable := &u.query.Output
	subquery := txTable.WithContext(ctx).Select(txTable.TransactionID).Where(txTable.TxID.Eq(txID))

	_, err = outTable.WithContext(ctx).
		Where(field.ContainsSubQuery([]field.Expr{outTable.TransactionID}, subquery.UnderlyingDB())).
		Where(outTable.SpentBy.IsNull()).
		UpdateSimple(outTable.Spendable.Value(true))
	if err != nil {
		return fmt.Errorf("failed to make outputs spendable by txID: %q: %w", txID, err)
	}

	return nil
}
