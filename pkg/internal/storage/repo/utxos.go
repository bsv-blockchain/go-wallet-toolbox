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

// skipLockedOption is the clause.Locking Options value appended on Postgres/MySQL so concurrent
// funders lock non-overlapping UTXO sets; SQLite omits row locks (it serializes writes).
const skipLockedOption = "SKIP LOCKED"

// FindNotReservedUTXOsForUpdate selects not-reserved UTXOs within the provided DB transaction.
// On Postgres/MySQL it appends SELECT FOR UPDATE SKIP LOCKED so concurrent callers automatically
// pick non-overlapping UTXO sets. On SQLite the lock is omitted since SQLite serializes writes;
// the guarded UPDATE in CreateTransactionInTx handles contention there instead.
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

	page.ApplyDefaults()

	statuses := []string{string(wdk.UTXOStatusMined), string(wdk.UTXOStatusUnproven)}
	if includeSending {
		statuses = append(statuses, string(wdk.UTXOStatusSending))
	}

	// Order by safety tier (mined=0, unproven=1, sending=2) then satoshis ascending
	// so the collector picks the safest, smallest-sufficient UTXOs first. output_id ASC
	// is appended as a unique tiebreaker: without it, rows sharing the same tier+satoshis
	// have no stable relative order, so under concurrent lock churn (rows locked/unlocked
	// between pages by SKIP LOCKED) a row can shift across the OFFSET boundary and be
	// returned on two different pages of the same scan.
	orderClause := fmt.Sprintf(
		"CASE utxo_status WHEN '%s' THEN 0 WHEN '%s' THEN 1 WHEN '%s' THEN 2 END ASC, satoshis ASC, output_id ASC",
		wdk.UTXOStatusMined, wdk.UTXOStatusUnproven, wdk.UTXOStatusSending,
	)

	query := tx.WithContext(ctx).Scopes(
		scopes.UserID(userID),
		scopes.BasketName(basketName),
		notReserved(),
		outputNotIn(forbiddenOutputIDs),
	).Where(u.query.UserUTXO.UTXOStatus.In(statuses...)).
		Order(orderClause).Offset(page.Offset).Limit(page.Limit)

	if tx.Name() != "sqlite" {
		query = query.Clauses(clause.Locking{Strength: "UPDATE", Options: skipLockedOption})
	}

	var result []*models.UserUTXO
	err = query.Find(&result).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find and lock not reserved UTXOs: %w", err)
	}
	return result, nil
}

// FindSmallestSufficientUTXOForUpdate returns the single not-reserved UTXO in the given
// status tier with the smallest satoshis >= minSatoshis (ties broken by output_id ASC),
// or (nil, nil) when no such row exists. The first row of this ordering is the exact
// match when one exists, else the smallest sufficient — replacing stages 1+2 of the old
// in-memory selectBest. On Postgres/MySQL the row is locked with SELECT ... FOR UPDATE
// SKIP LOCKED so concurrent callers automatically pick non-overlapping UTXOs; on SQLite
// the lock is omitted since SQLite serializes writes (see FindNotReservedUTXOsForUpdate).
func (u *UTXOs) FindSmallestSufficientUTXOForUpdate(
	ctx context.Context,
	tx *gorm.DB,
	q wdk.BoundedUTXOQuery,
	minSatoshis uint64,
) (*models.UserUTXO, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Utxos-FindSmallestSufficientUTXOForUpdate", attribute.Int("UserID", q.UserID), attribute.String("BasketName", q.BasketName), attribute.String("UTXOStatus", string(q.Status)))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	query := tx.WithContext(ctx).Scopes(
		scopes.UserID(q.UserID),
		scopes.BasketName(q.BasketName),
		notReserved(),
		outputNotIn(q.ExcludedOutputIDs),
	).Where("utxo_status = ?", string(q.Status)).
		Where("satoshis >= ?", minSatoshis).
		Order("satoshis ASC, output_id ASC").
		Limit(1)

	if tx.Name() != "sqlite" {
		query = query.Clauses(clause.Locking{Strength: "UPDATE", Options: skipLockedOption})
	}

	var result []*models.UserUTXO
	err = query.Find(&result).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find and lock smallest sufficient UTXO: %w", err)
	}
	if len(result) == 0 {
		return nil, nil
	}
	return result[0], nil
}

// FindLargestInsufficientUTXOsForUpdate returns up to limit not-reserved UTXOs in the
// given status tier with satoshis < maxSatoshis, largest first (satoshis DESC, ties
// broken by output_id DESC) — the batched replacement for stage 3 of the old in-memory
// selectBest (largest insufficient). An empty slice with a nil error means the tier has
// no eligible rows below the bound. Locking behavior matches
// FindSmallestSufficientUTXOForUpdate.
func (u *UTXOs) FindLargestInsufficientUTXOsForUpdate(
	ctx context.Context,
	tx *gorm.DB,
	q wdk.BoundedUTXOQuery,
	maxSatoshis uint64,
	limit int,
) ([]*models.UserUTXO, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Utxos-FindLargestInsufficientUTXOsForUpdate", attribute.Int("UserID", q.UserID), attribute.String("BasketName", q.BasketName), attribute.String("UTXOStatus", string(q.Status)))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	// Guard against GORM's Limit(0) footgun (interpreted as "no limit", which would
	// lock the whole tier). The only production caller passes insufficientBatchSize=16,
	// but this method is part of the shared interface — reject non-positive limits so a
	// future caller can't reintroduce whole-tier locking.
	if limit <= 0 {
		err = fmt.Errorf("limit must be positive, got %d", limit)
		return nil, err
	}

	query := tx.WithContext(ctx).Scopes(
		scopes.UserID(q.UserID),
		scopes.BasketName(q.BasketName),
		notReserved(),
		outputNotIn(q.ExcludedOutputIDs),
	).Where("utxo_status = ?", string(q.Status)).
		Where("satoshis < ?", maxSatoshis).
		Order("satoshis DESC, output_id DESC").
		Limit(limit)

	if tx.Name() != "sqlite" {
		query = query.Clauses(clause.Locking{Strength: "UPDATE", Options: skipLockedOption})
	}

	var result []*models.UserUTXO
	err = query.Find(&result).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find and lock largest insufficient UTXOs: %w", err)
	}
	return result, nil
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

// DeleteIndexRowsForOutputs removes UserUTXO (fundable index) rows for the given
// outputs. Used to drop rows that advertise already-spent outputs, which would
// otherwise be re-selected by every funding attempt and fail deterministically.
func (u *UTXOs) DeleteIndexRowsForOutputs(ctx context.Context, userID int, outputIDs []uint) error {
	if len(outputIDs) == 0 {
		return nil
	}

	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Utxos-DeleteIndexRowsForOutputs", attribute.Int("UserID", userID))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	utxoTable := &u.query.UserUTXO
	_, err = utxoTable.WithContext(ctx).
		Where(utxoTable.UserID.Eq(userID)).
		Where(utxoTable.OutputID.In(outputIDs...)).
		Delete()
	if err != nil {
		return fmt.Errorf("failed to delete stale utxo index rows: %w", err)
	}
	return nil
}

// MakeChangeSpendableAndIndexByTxID marks a transaction's change outputs
// spendable and (re)indexes them in UserUTXO with the transaction's current
// status tier.
//
// This exists for the delayed-broadcast queue path. Outputs are created
// not-spendable and are normally only flipped once the network accepts the
// transaction; until then their UserUTXO rows carry the placeholder (empty)
// status, which callers listing basket outputs count as inventory but the
// funder — which claims from UserUTXO by status tier — cannot see. At high
// request rates the acceptance pipeline lags far behind creation, so freshly
// minted change (notably the fuel pool) stayed unusable and funding failed
// with "not enough funds" against an apparently full basket.
//
// Promoting at queue time makes the outputs claimable at the "sending" tier,
// which only spend policies that opt into unaccepted funds (SpendPolicyAny)
// will select — the same trade-off AcceptDelayedBroadcast already implies.
// Callers must NOT use this for no-send transactions, whose change is reachable
// only through an explicit noSendChange reference.
func (u *UTXOs) MakeChangeSpendableAndIndexByTxID(ctx context.Context, txID string) error {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Utxos-MakeChangeSpendableAndIndexByTxID", attribute.String("TxID", txID))
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
				// Never touch an output that is already spent: makeOutputsSpendable
				// also clears spent_by, so without this guard a repeated call
				// would resurrect a spent output and index it as fundable —
				// producing exactly the stale index rows that poison funding.
				Where(query.Output.SpentBy.IsNull()).
				Scopes(isChangeDaoScope(query))
		}

		if spendableErr := makeOutputsSpendable(ctx, query, filterScope); spendableErr != nil {
			return spendableErr
		}

		changeOutputs, outputsErr := getOutputsWithTxStatus(ctx, query, filterScope)
		if outputsErr != nil {
			return outputsErr
		}

		return createUTXOsFromOutputs(ctx, query, changeOutputs)
	})
	if err != nil {
		return fmt.Errorf("failed to make change spendable for txID %q: %w", txID, err)
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
		return createUTXOsForSpendableOutputsByTxID(ctx, query, txID)
	})
	if err != nil {
		return fmt.Errorf("failed to make outputs spendable by txID: %q: %w", txID, err)
	}

	return nil
}

// createUTXOsForSpendableOutputsByTxID creates UserUTXOs for the spendable change
// outputs of the transactions with the given txID using the caller-provided
// (transaction-bound) query, so that callers composing larger atomic flows can run
// it inside their own database transaction.
func createUTXOsForSpendableOutputsByTxID(ctx context.Context, query *genquery.Query, txID string) error {
	filterScope := func(dao gen.Dao) gen.Dao {
		subquery := query.Transaction.
			Select(query.Transaction.ID).
			Where(query.Transaction.TxID.Eq(txID))

		return dao.
			Where(field.ContainsSubQuery([]field.Expr{query.Output.TransactionID}, subquery.UnderlyingDB())).
			Where(query.Output.Spendable.Is(true)).
			Scopes(isChangeDaoScope(query))
	}

	changeOutputs, err := getOutputsWithTxStatus(ctx, query, filterScope)
	if err != nil {
		return err
	}

	if err := createUTXOsFromOutputs(ctx, query, changeOutputs); err != nil {
		return err
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
