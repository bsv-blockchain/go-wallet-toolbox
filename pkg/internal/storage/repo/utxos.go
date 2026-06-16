package repo

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"gorm.io/gen"
	"gorm.io/gen/field"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/genquery"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/scopes"
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

func (u *UTXOs) FindNotReservedUTXOs(
	ctx context.Context,
	userID int,
	basketName string,
	page *queryopts.Paging,
	forbiddenOutputIDs []uint,
	includeSending bool,
) ([]*models.UserUTXO, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Utxos-FindNotReservedUTXOs", attribute.Int("UserID", userID), attribute.String("BasketName", basketName), attribute.Bool("IncludeSending", includeSending))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	var result []*models.UserUTXO
	err = u.buildNotReservedQuery(u.db.WithContext(ctx), userID, basketName, page, forbiddenOutputIDs, includeSending, false).Find(&result).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find not reserved UTXOs: %w", err)
	}
	return result, nil
}

// FindNotReservedUTXOsForUpdate is like FindNotReservedUTXOs but executes within the provided DB
// transaction and adds SELECT FOR UPDATE SKIP LOCKED for Postgres/MySQL, preventing concurrent
// goroutines from selecting the same UTXOs. For SQLite the lock is omitted since SQLite serializes
// writes; the guarded UPDATE in CreateTransactionInTx handles contention there instead.
func (u *UTXOs) FindNotReservedUTXOsForUpdate(
	ctx context.Context,
	tx *gorm.DB,
	userID int,
	basketName string,
	page *queryopts.Paging,
	forbiddenOutputIDs []uint,
	includeSending bool,
) ([]*models.UserUTXO, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Utxos-FindNotReservedUTXOsForUpdate", attribute.Int("UserID", userID), attribute.String("BasketName", basketName), attribute.Bool("IncludeSending", includeSending))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	forUpdate := tx.Name() != "sqlite"
	var result []*models.UserUTXO
	err = u.buildNotReservedQuery(tx.WithContext(ctx), userID, basketName, page, forbiddenOutputIDs, includeSending, forUpdate).Find(&result).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find and lock not reserved UTXOs: %w", err)
	}
	return result, nil
}

// buildNotReservedQuery constructs the shared base query for both FindNotReservedUTXOs variants.
// When forUpdate is true, SELECT FOR UPDATE SKIP LOCKED is appended (Postgres/MySQL only).
func (u *UTXOs) buildNotReservedQuery(
	db *gorm.DB,
	userID int,
	basketName string,
	page *queryopts.Paging,
	forbiddenOutputIDs []uint,
	includeSending bool,
	forUpdate bool,
) *gorm.DB {
	page.ApplyDefaults()

	statuses := []string{string(wdk.UTXOStatusMined), string(wdk.UTXOStatusUnproven)}
	if includeSending {
		statuses = append(statuses, string(wdk.UTXOStatusSending))
	}

	// Order by safety tier (mined=0, unproven=1, sending=2) then by satoshis ascending
	// so the collector picks the safest, smallest-sufficient UTXOs first.
	orderClause := fmt.Sprintf(
		"CASE utxo_status WHEN '%s' THEN 0 WHEN '%s' THEN 1 WHEN '%s' THEN 2 END ASC, satoshis ASC",
		wdk.UTXOStatusMined, wdk.UTXOStatusUnproven, wdk.UTXOStatusSending,
	)

	q := db.Scopes(
		scopes.UserID(userID),
		scopes.BasketName(basketName),
		notReserved(),
		outputNotIn(forbiddenOutputIDs),
	).Where(u.query.UserUTXO.UTXOStatus.In(statuses...)).
		Order(orderClause).Offset(page.Offset).Limit(page.Limit)

	if forUpdate {
		q = q.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"})
	}
	return q
}

func (u *UTXOs) CountUTXOs(ctx context.Context, userID int, basketName string) (int64, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Utxos-CountUTXOs", attribute.Int("UserID", userID), attribute.String("BasketName", basketName))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	count := int64(0)

	err = u.db.WithContext(ctx).
		Model(&models.UserUTXO{}).
		Scopes(scopes.UserID(userID), scopes.BasketName(basketName), notReserved()).
		Count(&count).Error

	return count, err
}

func (u *UTXOs) UnreserveUTXOsByTransactionID(ctx context.Context, transactionID uint) error {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Utxos-UnreserveUTXOsByTransactionID", attribute.String("TransactionID", fmt.Sprintf("%d", transactionID)))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	table := u.query.UserUTXO
	_, err = table.WithContext(ctx).
		Where(table.ReservedByID.Eq(transactionID)).
		Update(table.ReservedByID, nil)
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

	err = u.query.DBTransaction(func(query *genquery.Query) error {
		filterScope := func(dao gen.Dao) gen.Dao {
			subquery := query.Transaction.
				Select(query.Transaction.ID).
				Where(query.Transaction.TxID.Eq(txID))

			return dao.
				Where(field.ContainsSubQuery([]field.Expr{query.Output.TransactionID}, subquery.UnderlyingDB())).
				Where(query.Output.Spendable.Is(true)).
				Scopes(isChangeDaoScope(query))
		}

		var changeOutputs []*outputWithTxStatus
		changeOutputs, err = getOutputsWithTxStatus(ctx, query, filterScope)
		if err != nil {
			return err
		}

		err = createUTXOsFromOutputs(ctx, query, changeOutputs)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to make outputs spendable by txID: %q: %w", txID, err)
	}

	return nil
}

func notReserved() func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("reserved_by_id IS NULL")
	}
}

func outputNotIn(forbiddenOutputIDs []uint) func(*gorm.DB) *gorm.DB {
	if len(forbiddenOutputIDs) == 0 {
		return func(db *gorm.DB) *gorm.DB {
			return db
		}
	}
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("output_id NOT IN ?", forbiddenOutputIDs)
	}
}
