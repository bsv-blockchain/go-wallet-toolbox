package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-softwarelab/common/pkg/seq"
	"github.com/go-softwarelab/common/pkg/seq2"
	"github.com/go-softwarelab/common/pkg/slices"
	"go.opentelemetry.io/otel/attribute"
	"gorm.io/gen"
	"gorm.io/gorm"

	pkgentity "github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/genquery"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/scopes"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/dbretry"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/history"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/tracing"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

const errFmtDBTransaction = "db transaction failed: %w"

// statusColumn is the shared column name used by the status writers.
const statusColumn = "status"

const (
	maxDepthOfRecursion = 1000
)

type KnownTx struct {
	db    *gorm.DB
	query *genquery.Query
	retry *dbretry.Policy
}

func NewKnownTxRepo(db *gorm.DB, query *genquery.Query, retry *dbretry.Policy) *KnownTx {
	return &KnownTx{db: db, query: query, retry: retry}
}

func (p *KnownTx) UpsertKnownTx(ctx context.Context, req *entity.UpsertKnownTx, txNote history.Builder) error {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-KnownTx-UpsertKnownTx", attribute.String("TxID", req.TxID))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	err = runInTransaction(ctx, p.db, p.retry, func(tx *gorm.DB) error {
		return upsertKnownTx(tx, req, txNote)
	})
	if err != nil {
		return fmt.Errorf("failed to upsert known tx: %w", err)
	}
	return nil
}

func (p *KnownTx) UpdateKnownTxStatus(ctx context.Context, txID string, status wdk.ProvenTxReqStatus, skipForStatuses []wdk.ProvenTxReqStatus, txNotes []history.Builder) error {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-KnownTx-UpdateKnownTxStatus", attribute.String("TxID", txID), attribute.String("Status", string(status)))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	return updateKnownTxStatus(p.db.WithContext(ctx), txID, status, skipForStatuses, txNotes)
}

// KnownTxNeverPostedStatuses are the KnownTx statuses that carry no post attempt yet:
// the transaction is queued or freshly recorded, but was never handed to a broadcaster.
var KnownTxNeverPostedStatuses = []wdk.ProvenTxReqStatus{
	wdk.ProvenTxStatusUnprocessed,
	wdk.ProvenTxStatusUnsent,
	wdk.ProvenTxStatusNoSend,
}

// ParkUnbroadcastKnownTx marks a KnownTx as invalidTx so that no broadcast pipeline picks
// it up again. It is guarded to transactions that provably never reached the network:
// a never-posted status, was_broadcast still false and no recorded attempt. Returns
// applied=false (writing nothing) when the guard holds.
//
// This is the KnownTx half of a pre-broadcast abort: releasing the reserved inputs while
// leaving the shared KnownTx broadcastable would let send_waiting post a transaction whose
// inputs are spendable again - a real double spend.
func (p *KnownTx) ParkUnbroadcastKnownTx(ctx context.Context, txID string, txNotes []history.Builder) (bool, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-KnownTx-ParkUnbroadcastKnownTx", attribute.String("TxID", txID))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	applied := false
	err = runInTransaction(ctx, p.db, p.retry, func(tx *gorm.DB) error {
		result := tx.Model(&models.KnownTx{}).
			Where("tx_id = ?", txID).
			Where("status IN ?", KnownTxNeverPostedStatuses).
			Where("was_broadcast = ?", false).
			Where("attempts = ?", 0).
			UpdateColumns(map[string]any{
				statusColumn: wdk.ProvenTxStatusInvalid,
			})
		if result.Error != nil {
			return fmt.Errorf("failed to park known tx %s: %w", txID, result.Error)
		}
		if result.RowsAffected == 0 {
			return nil
		}
		applied = true

		return addTxNotes(tx, slices.Map(txNotes, func(note history.Builder) *pkgentity.TxHistoryNote {
			return note.Entity(txID)
		}))
	})
	if err != nil {
		return false, err
	}

	return applied, nil
}

// KnownTxClaimableForBroadcastStatuses are the statuses a transaction can be claimed for a
// post from: it is waiting to be sent, explicitly parked by its owner (noSend, sent later via
// sendWith), or a previous attempt ended without network evidence and is being retried.
//
// Everything else is excluded on purpose: statuses that already carry network evidence, the
// terminal failures - which is what an abort parks a transaction as - and 'unfail', which is
// re-checking an earlier failure.
var KnownTxClaimableForBroadcastStatuses = []wdk.ProvenTxReqStatus{
	wdk.ProvenTxStatusUnprocessed,
	wdk.ProvenTxStatusUnsent,
	wdk.ProvenTxStatusSending,
	wdk.ProvenTxStatusNoSend,
	wdk.ProvenTxStatusNonFinal,
	wdk.ProvenTxStatusUnknown,
}

// ClaimKnownTxsForBroadcast takes ownership of the given transactions for an imminent post:
// it moves every claimable row to 'sending' (flagging was_broadcast) and returns the txIDs
// that were actually claimed.
//
// It is the gate that makes a parked KnownTx an effective stop signal: a transaction that
// was aborted (or otherwise moved out of a claimable status) is not returned, so callers
// must not post it. Claiming immediately before the post also prevents a queued in-memory
// broadcast from racing an abort that already released the transaction's inputs.
func (p *KnownTx) ClaimKnownTxsForBroadcast(ctx context.Context, txIDs []string) ([]string, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-KnownTx-ClaimKnownTxsForBroadcast")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	if len(txIDs) == 0 {
		return nil, nil
	}

	var claimed []string
	err = runInTransaction(ctx, p.db, p.retry, func(tx *gorm.DB) error {
		var rows []*models.KnownTx
		if findErr := tx.Model(&models.KnownTx{}).
			Select("tx_id").
			Where("tx_id IN ?", txIDs).
			Where(statusInCondition, KnownTxClaimableForBroadcastStatuses).
			Find(&rows).Error; findErr != nil {
			return fmt.Errorf("failed to find claimable known txs: %w", findErr)
		}

		claimed = slices.Map(rows, func(row *models.KnownTx) string { return row.TxID })
		if len(claimed) == 0 {
			return nil
		}

		if updateErr := tx.Model(&models.KnownTx{}).
			Where("tx_id IN ?", claimed).
			Where(statusInCondition, KnownTxClaimableForBroadcastStatuses).
			UpdateColumns(map[string]any{
				statusColumn:    wdk.ProvenTxStatusSending,
				"was_broadcast": true,
			}).Error; updateErr != nil {
			return fmt.Errorf("failed to claim known txs for broadcast: %w", updateErr)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return claimed, nil
}

func upsertKnownTx(tx *gorm.DB, req *entity.UpsertKnownTx, txNote history.Builder) error {
	var model models.KnownTx
	err := tx.First(&model, "tx_id = ? ", req.TxID).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("cannot upsert known tx: %w", err)
	}

	if len(req.SkipForStatuses) > 0 {
		for _, skipStatus := range req.SkipForStatuses {
			if model.Status == skipStatus {
				return nil
			}
		}
	}

	model.Status = req.Status
	model.TxID = req.TxID
	model.RawTx = req.RawTx
	model.InputBeef = req.InputBeef
	model.WasBroadcast = model.WasBroadcast || req.Status.WasBroadcastStatus()

	err = tx.Save(&model).Error
	if err != nil {
		return fmt.Errorf("cannot save known tx: %w", err)
	}

	err = addTxNote(tx, txNote.Entity(req.TxID))
	if err != nil {
		return err
	}

	return nil
}

// FailKnownTxAsDoubleSpend atomically applies the terminal double-spend failure for a
// tx rejected by the network. The guarded KnownTx -> doubleSpend transition is evaluated
// by the database FIRST (skipForStatuses in the UPDATE's WHERE clause), and the dependent
// writes (Transactions -> failed, their spent outputs restored) execute only when that
// transition actually applied. When the guard holds - e.g. the tx completed concurrently
// while the rejection was being re-verified against the network - nothing is written and
// applied=false is returned.
func (p *KnownTx) FailKnownTxAsDoubleSpend(ctx context.Context, txID string, skipForStatuses []wdk.ProvenTxReqStatus, txNotes []history.Builder) (applied bool, err error) {
	ctx, span := tracing.StartTracing(ctx, "Repository-KnownTx-FailKnownTxAsDoubleSpend", attribute.String("TxID", txID))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	err = runInTransaction(ctx, p.db, p.retry, func(tx *gorm.DB) error {
		var txErr error
		applied, txErr = applyKnownTxStatusGuarded(tx, txID, wdk.ProvenTxStatusDoubleSpend, skipForStatuses, txNotes)
		if txErr != nil || !applied {
			return txErr
		}

		txErr = tx.Model(&models.Transaction{}).
			Where(p.query.Transaction.TxID.Eq(txID)).
			Updates(map[string]any{
				p.query.Transaction.Status.ColumnName().String(): wdk.TxStatusFailed,
			}).Error
		if txErr != nil {
			return fmt.Errorf("failed to update transaction status as failed: %w", txErr)
		}

		// Spent inputs are intentionally NOT restored to spendable here: a confirmed
		// double spend only proves a conflicting tx exists, not that this tx's original
		// spend is invalid. Restoring the input risks the wallet re-spending it and
		// creating a real double spend on top of the reported one, so the input is left
		// claimed (and effectively lost) rather than resurrected.

		return nil
	})
	if err != nil {
		return false, fmt.Errorf(errFmtDBTransaction, err)
	}
	return applied, nil
}

// AdvanceKnownTxToBroadcasted atomically advances a tx to the post-broadcast state the
// broadcast flow sets after a successful broadcast. The guarded KnownTx -> unmined
// transition is evaluated by the database FIRST (skipForStatuses in the UPDATE's WHERE
// clause), and the dependent writes (Transactions -> unproven, UTXOs for spendable change
// outputs) execute only when that transition actually applied. When the guard holds
// (e.g. the tx advanced concurrently) nothing is written and applied=false is returned.
func (p *KnownTx) AdvanceKnownTxToBroadcasted(ctx context.Context, txID string, skipForStatuses []wdk.ProvenTxReqStatus, txNotes []history.Builder) (applied bool, err error) {
	ctx, span := tracing.StartTracing(ctx, "Repository-KnownTx-AdvanceKnownTxToBroadcasted", attribute.String("TxID", txID))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	err = runInTransaction(ctx, p.db, p.retry, func(tx *gorm.DB) error {
		var txErr error
		applied, txErr = applyKnownTxStatusGuarded(tx, txID, wdk.ProvenTxStatusUnmined, skipForStatuses, txNotes)
		if txErr != nil || !applied {
			return txErr
		}

		txErr = tx.Model(&models.Transaction{}).
			Where(p.query.Transaction.TxID.Eq(txID)).
			Updates(map[string]any{
				p.query.Transaction.Status.ColumnName().String(): wdk.TxStatusUnproven,
			}).Error
		if txErr != nil {
			return fmt.Errorf("failed to update transaction status as unproven: %w", txErr)
		}

		if txErr = createUTXOsForSpendableOutputsByTxID(ctx, genquery.Use(tx), txID); txErr != nil {
			return fmt.Errorf("failed to make outputs spendable: %w", txErr)
		}

		return nil
	})
	if err != nil {
		return false, fmt.Errorf(errFmtDBTransaction, err)
	}
	return applied, nil
}

// applyKnownTxStatusGuarded is updateKnownTxStatus reporting whether the guarded update
// actually changed the row: skipForStatuses is evaluated by the database inside the
// UPDATE's WHERE clause, so concurrent transitions into a skipped status (e.g. completed)
// are detected reliably. Notes are recorded only when the update applied.
func applyKnownTxStatusGuarded(tx *gorm.DB, txID string, status wdk.ProvenTxReqStatus, skipForStatuses []wdk.ProvenTxReqStatus, txNotes []history.Builder) (bool, error) {
	query := tx.Model(&models.KnownTx{}).Where("tx_id = ? ", txID)
	if len(skipForStatuses) > 0 {
		query = query.Where("status NOT IN ? ", skipForStatuses)
	}

	updates := map[string]any{
		"status": status,
	}
	if status.WasBroadcastStatus() {
		updates["was_broadcast"] = true
	}

	result := query.UpdateColumns(updates)
	if result.Error != nil {
		return false, fmt.Errorf("failed to update known tx status: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return false, nil
	}

	err := addTxNotes(tx, slices.Map(txNotes, func(note history.Builder) *pkgentity.TxHistoryNote {
		return note.Entity(txID)
	}))
	if err != nil {
		return false, err
	}

	return true, nil
}

func updateKnownTxStatus(tx *gorm.DB, txID string, status wdk.ProvenTxReqStatus, skipForStatuses []wdk.ProvenTxReqStatus, txNotes []history.Builder) error {
	var model models.KnownTx

	query := tx.Model(&model).Where("tx_id = ? ", txID)
	if len(skipForStatuses) > 0 {
		query = query.Where("status NOT IN ? ", skipForStatuses)
	}

	updates := map[string]any{
		"status": status,
	}
	if status.WasBroadcastStatus() {
		updates["was_broadcast"] = true
	}

	result := query.UpdateColumns(updates)
	if result.Error != nil {
		return fmt.Errorf("failed to update known tx status: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("%w: known tx %s -> %s", ErrStatusUpdateSkipped, txID, status)
	}

	// History notes are recorded only for transitions that actually matched a row;
	// a skipped update (skip-list or absent row) must not leave misleading notes.
	err := addTxNotes(tx, slices.Map(txNotes, func(note history.Builder) *pkgentity.TxHistoryNote {
		return note.Entity(txID)
	}))
	if err != nil {
		return err
	}

	return nil
}

func (p *KnownTx) FindKnownTxRawTx(ctx context.Context, txID string) ([]byte, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-KnownTx-FindKnownTxRawTx", attribute.String("TxID", txID))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	var model models.KnownTx
	err = p.db.WithContext(ctx).
		Model(&model).
		Select("raw_tx").
		First(&model, "tx_id = ? ", txID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find raw tx of known tx: %w", err)
	}
	return model.RawTx, nil
}

func (p *KnownTx) FindKnownTxRawTxs(ctx context.Context, txIDs []string) (map[string][]byte, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-KnownTx-FindKnownTxRawTx", attribute.StringSlice("TxIDs", txIDs))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	if len(txIDs) == 0 {
		return make(map[string][]byte), nil
	}

	var results []struct {
		TxID  string
		RawTx []byte
	}

	err = p.db.WithContext(ctx).
		Model(&models.KnownTx{}).
		Select("tx_id, raw_tx").
		Where("tx_id IN ?", txIDs).
		Scan(&results).Error
	if err != nil {
		return nil, fmt.Errorf("failed to batch fetch raw tx: %w", err)
	}

	rawTxMap := make(map[string][]byte)
	for _, r := range results {
		rawTxMap[r.TxID] = r.RawTx
	}
	return rawTxMap, nil
}

func (p *KnownTx) FindKnownTxStatuses(ctx context.Context, txIDs ...string) (map[string]wdk.ProvenTxReqStatus, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-KnownTx-FindKnownTxStatuses", attribute.StringSlice("TxIDs", txIDs))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	var rows []*models.KnownTx
	err = p.db.WithContext(ctx).
		Model(&models.KnownTx{}).
		Select("status, tx_id").
		Where("tx_id IN (?)", txIDs).
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find proven tx statuses for list of txIDs: %w", err)
	}

	txIDStatuses := seq.MapTo(seq.FromSlice(rows), func(row *models.KnownTx) (string, wdk.ProvenTxReqStatus) {
		return row.TxID, row.Status
	})

	return seq2.CollectToMap(txIDStatuses), nil
}

func (p *KnownTx) AllKnownTxsExist(ctx context.Context, txIDs []string, sourceTxsStatusFilter []wdk.ProvenTxReqStatus) (bool, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-KnownTx-AllKnownTxsExist", attribute.StringSlice("TxIDs", txIDs))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	var model models.KnownTx
	query := p.db.WithContext(ctx).
		Model(&model).
		Select("tx_id").
		Where("tx_id IN (?) ", txIDs).
		Where("raw_tx IS NOT NULL").
		Where("LENGTH(raw_tx) > 0").
		Where("input_beef IS NOT NULL").
		Where("LENGTH(input_beef) > 0")

	if len(sourceTxsStatusFilter) > 0 {
		query = query.Where("status IN ? ", sourceTxsStatusFilter)
	}

	var count int64
	err = query.Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("failed to check if known transactions exist: %w", err)
	}

	return count == int64(len(txIDs)), nil
}

func (p *KnownTx) FindKnownTxIDsByStatuses(ctx context.Context, txStatus []wdk.ProvenTxReqStatus, opts ...queryopts.Options) ([]*entity.KnownTxForStatusSync, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-KnownTx-FindKnownTxIDsByStatuses")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	var rows []*models.KnownTx
	err = p.db.WithContext(ctx).
		Model(&models.KnownTx{}).
		Select("tx_id, status, attempts, was_broadcast, rebroadcast_attempts, batch").
		Scopes(scopes.FromQueryOpts(opts)...).
		Where("status IN ? ", txStatus).
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find known tx ids by statuses: %w", err)
	}

	return mapKnownTxRowsForStatusSync(rows), nil
}

func (p *KnownTx) FindKnownTxIDsReadyForStatusSync(ctx context.Context, txStatus []wdk.ProvenTxReqStatus, opts ...queryopts.Options) ([]*entity.KnownTxForStatusSync, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-KnownTx-FindKnownTxIDsReadyForStatusSync")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	var rows []*models.KnownTx
	query := p.db.WithContext(ctx).
		Model(&models.KnownTx{}).
		Select("tx_id, status, attempts, was_broadcast, rebroadcast_attempts, batch").
		Scopes(scopes.FromQueryOpts(opts)...)
	query = withReadyForStatusSyncFilter(query, txStatus)

	err = query.Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find known tx ids ready for status sync: %w", err)
	}

	return mapKnownTxRowsForStatusSync(rows), nil
}

func withReadyForStatusSyncFilter(query *gorm.DB, txStatus []wdk.ProvenTxReqStatus) *gorm.DB {
	statusesWithoutUnsent := make([]wdk.ProvenTxReqStatus, 0, len(txStatus))
	for _, status := range txStatus {
		if status == wdk.ProvenTxStatusUnsent {
			continue
		}
		statusesWithoutUnsent = append(statusesWithoutUnsent, status)
	}

	if len(statusesWithoutUnsent) == 0 {
		return query.Where("status = ? AND was_broadcast = ?", wdk.ProvenTxStatusUnsent, true)
	}

	return query.Where(
		"(status IN ? OR (status = ? AND was_broadcast = ?))",
		statusesWithoutUnsent,
		wdk.ProvenTxStatusUnsent,
		true,
	)
}

func (p *KnownTx) FindKnownTxIDsByStatusesNeedingFailureReview(ctx context.Context, txStatus []wdk.ProvenTxReqStatus, limit int) ([]*entity.KnownTxForStatusSync, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-KnownTx-FindKnownTxIDsByStatusesNeedingFailureReview")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	if len(txStatus) == 0 {
		return nil, nil
	}

	if limit <= 0 {
		limit = 1000
	}

	knownTxTable := p.query.KnownTx.TableName()
	transactionTable := p.query.Transaction.TableName()
	outputTable := p.query.Output.TableName()

	var rows []*models.KnownTx
	err = p.db.WithContext(ctx).
		Model(&models.KnownTx{}).
		Select("tx_id, status, attempts, was_broadcast, rebroadcast_attempts, batch").
		Where("status IN ? ", txStatus).
		Where(fmt.Sprintf(`
			EXISTS (
				SELECT 1
				FROM %s
				LEFT JOIN %s
					ON %s.spent_by = %s.id
					AND %s.deleted_at IS NULL
				WHERE %s.tx_id = %s.tx_id
					AND %s.deleted_at IS NULL
					AND (%s.status <> ? OR %s.id IS NOT NULL)
			)
		`, transactionTable, outputTable, outputTable, transactionTable, outputTable, transactionTable, knownTxTable, transactionTable, transactionTable, outputTable), wdk.TxStatusFailed).
		Order(fmt.Sprintf("%s.created_at ASC", knownTxTable)).
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find failed known tx ids needing review: %w", err)
	}

	return mapKnownTxRowsForStatusSync(rows), nil
}

func mapKnownTxRowsForStatusSync(rows []*models.KnownTx) []*entity.KnownTxForStatusSync {
	return slices.Map(rows, func(row *models.KnownTx) *entity.KnownTxForStatusSync {
		return &entity.KnownTxForStatusSync{
			TxID:                row.TxID,
			Attempts:            row.Attempts,
			RebroadcastAttempts: row.RebroadcastAttempts,
			Status:              row.Status,
			WasBroadcast:        row.WasBroadcast || row.Status.WasBroadcastStatus(),
			Batch:               row.Batch,
		}
	})
}

func (p *KnownTx) UpdateKnownTxAsMined(ctx context.Context, knownTxAsMined *entity.KnownTxAsMined) error {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-KnownTx-UpdateKnownTxAsMined", attribute.String("TxID", knownTxAsMined.TxID))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	err = runInTransaction(ctx, p.db, p.retry, func(tx *gorm.DB) error {
		err = tx.Model(&models.KnownTx{}).
			Where(p.query.KnownTx.TxID.Eq(knownTxAsMined.TxID)).
			Updates(&models.KnownTx{
				Status:       wdk.ProvenTxStatusCompleted,
				WasBroadcast: true,
				BlockHash:    &knownTxAsMined.BlockHash,
				BlockHeight:  &knownTxAsMined.BlockHeight,
				MerklePath:   knownTxAsMined.MerklePath,
				MerkleRoot:   &knownTxAsMined.MerkleRoot,
				Notified:     true,
			}).Error
		if err != nil {
			return fmt.Errorf("failed to update known tx: %w", err)
		}

		err = addTxNotes(tx, slices.Map(knownTxAsMined.Notes, func(note history.Builder) *pkgentity.TxHistoryNote {
			return note.Entity(knownTxAsMined.TxID)
		}))
		if err != nil {
			return fmt.Errorf("failed to add tx notes: %w", err)
		}

		// NOTE: There can be multiple transactions with the same tx_id, so we need to update all of them.
		err = tx.Model(&models.Transaction{}).
			Where(p.query.Transaction.TxID.Eq(knownTxAsMined.TxID)).
			Updates(map[string]any{
				p.query.Transaction.Status.ColumnName().String(): wdk.TxStatusCompleted,
			}).Error
		if err != nil {
			return fmt.Errorf("failed to update transaction status as completed: %w", err)
		}

		err = tx.Model(&models.UserUTXO{}).
			Where(
				"output_id in (?)",
				tx.Model(&models.Output{}).
					Select("id").
					Where(
						"transaction_id in (?)",
						tx.Model(&models.Transaction{}).
							Select("id").
							Where(p.query.Transaction.TxID.Eq(knownTxAsMined.TxID)),
					),
			).
			Updates(map[string]any{
				p.query.UserUTXO.UTXOStatus.ColumnName().String(): wdk.UTXOStatusMined,
			}).Error
		if err != nil {
			return fmt.Errorf("failed to update user UTXO status as mined: %w", err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf(errFmtDBTransaction, err)
	}
	return nil
}

func (p *KnownTx) IncreaseKnownTxAttemptsForTxIDs(ctx context.Context, txIDs []string) error {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-KnownTx-IncreaseKnownTxAttemptsForTxIDs", attribute.StringSlice("TxIDs", txIDs))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	if len(txIDs) == 0 {
		return nil
	}

	err = p.db.WithContext(ctx).Model(&models.KnownTx{}).
		Where("tx_id IN ? ", txIDs).
		UpdateColumn("attempts", gorm.Expr("attempts + 1")).Error
	if err != nil {
		return fmt.Errorf("failed to increase attempts for tx ids: %w", err)
	}
	return nil
}

func (p *KnownTx) ApplyProofTimeouts(ctx context.Context, attempts, maxRebroadcastAttempts uint64, statuses []wdk.ProvenTxReqStatus) ([]models.KnownTx, error) {
	var (
		err        error
		updatedTxs []models.KnownTx
	)
	ctx, span := tracing.StartTracing(ctx, "Repository-KnownTx-ApplyProofTimeouts", attribute.String("Attempts", fmt.Sprintf("%d", attempts)))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	if attempts == 0 {
		return nil, nil
	}

	err = runInTransaction(ctx, p.db, p.retry, func(tx *gorm.DB) error {
		var timedOut []*models.KnownTx
		query := tx.Model(&models.KnownTx{}).
			Where("attempts >= ?", attempts)
		if len(statuses) > 0 {
			query = withReadyForStatusSyncFilter(query, statuses)
		}

		if findErr := query.
			Select("tx_id, status, attempts, was_broadcast, rebroadcast_attempts").
			Find(&timedOut).Error; findErr != nil {
			return fmt.Errorf("failed to find known transactions above attempts: %w", findErr)
		}

		updatedTxs = make([]models.KnownTx, 0, len(timedOut))
		for _, knownTx := range timedOut {
			updates := proofTimeoutUpdates(knownTx, maxRebroadcastAttempts)
			if updateErr := tx.Model(&models.KnownTx{}).
				Where("tx_id = ?", knownTx.TxID).
				UpdateColumns(updates).Error; updateErr != nil {
				return fmt.Errorf("failed to apply proof timeout for known transaction %s: %w", knownTx.TxID, updateErr)
			}

			knownTx.Status = updates["status"].(wdk.ProvenTxReqStatus)
			knownTx.Attempts = updates["attempts"].(uint64)
			knownTx.WasBroadcast = updates["was_broadcast"].(bool)
			knownTx.RebroadcastAttempts = updates["rebroadcast_attempts"].(uint64)
			updatedTxs = append(updatedTxs, *knownTx)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to apply proof timeouts: %w", err)
	}
	return updatedTxs, nil
}

// RequeueKnownTxForRebroadcast atomically demotes an in-flight broadcast tx back to the
// rebroadcast queue after an external (SSE) rejection that could not be confirmed as a
// double spend. The guarded transition applies only while the stored status is still one
// of fromStatuses (evaluated inside the UPDATE's WHERE clause), so a tx that completed
// concurrently is never clobbered. The rebroadcast budget mirrors ApplyProofTimeouts:
// within budget the tx returns to unsent (attempts reset, rebroadcast_attempts+1) for the
// send_waiting task to pick up; with the budget exhausted it fails terminally as invalid
// (the reviewKnownTxStatuses janitor cascades the user transaction failure).
func (p *KnownTx) RequeueKnownTxForRebroadcast(ctx context.Context, txID string, maxRebroadcastAttempts uint64, fromStatuses []wdk.ProvenTxReqStatus, txNotes []history.Builder) (newStatus wdk.ProvenTxReqStatus, applied bool, err error) {
	ctx, span := tracing.StartTracing(ctx, "Repository-KnownTx-RequeueKnownTxForRebroadcast", attribute.String("TxID", txID))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	err = runInTransaction(ctx, p.db, p.retry, func(tx *gorm.DB) error {
		var candidates []*models.KnownTx
		if findErr := tx.Model(&models.KnownTx{}).
			Where("tx_id = ? AND status IN ?", txID, fromStatuses).
			Select("tx_id, status, attempts, was_broadcast, rebroadcast_attempts").
			Limit(1).
			Find(&candidates).Error; findErr != nil {
			return fmt.Errorf("failed to find known transaction %s for rebroadcast requeue: %w", txID, findErr)
		}
		if len(candidates) == 0 {
			return nil
		}

		updates := proofTimeoutUpdates(candidates[0], maxRebroadcastAttempts)
		newStatus = updates["status"].(wdk.ProvenTxReqStatus)

		result := tx.Model(&models.KnownTx{}).
			Where("tx_id = ? AND status IN ?", txID, fromStatuses).
			UpdateColumns(updates)
		if result.Error != nil {
			return fmt.Errorf("failed to requeue known transaction %s for rebroadcast: %w", txID, result.Error)
		}
		if result.RowsAffected == 0 {
			return nil
		}
		applied = true

		return addTxNotes(tx, slices.Map(txNotes, func(note history.Builder) *pkgentity.TxHistoryNote {
			return note.Entity(txID)
		}))
	})
	if err != nil {
		return "", false, fmt.Errorf(errFmtDBTransaction, err)
	}
	return newStatus, applied, nil
}

func proofTimeoutUpdates(knownTx *models.KnownTx, maxRebroadcastAttempts uint64) map[string]any {
	wasBroadcast := knownTx.WasBroadcast || knownTx.Status.WasBroadcastStatus()
	if wasBroadcast && (maxRebroadcastAttempts == 0 || knownTx.RebroadcastAttempts < maxRebroadcastAttempts) {
		return map[string]any{
			"status":               wdk.ProvenTxStatusUnsent,
			"attempts":             uint64(0),
			"was_broadcast":        true,
			"rebroadcast_attempts": knownTx.RebroadcastAttempts + 1,
		}
	}

	return map[string]any{
		"status":               wdk.ProvenTxStatusInvalid,
		"attempts":             knownTx.Attempts,
		"was_broadcast":        wasBroadcast,
		"rebroadcast_attempts": knownTx.RebroadcastAttempts,
	}
}

func (p *KnownTx) FindKnownTxs(ctx context.Context, spec *pkgentity.KnownTxReadSpecification, opts ...queryopts.Options) ([]*pkgentity.KnownTx, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-KnownTx-FindKnownTxs")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	table := &p.query.KnownTx

	txNoteScope := func(dao gen.Dao) gen.Dao {
		if !spec.IncludeHistoryNotes {
			return dao
		}

		return dao.Preload(table.TxNotes)
	}

	scopesToApply := append(scopes.FromQueryOptsForGen(table, opts), txNoteScope)

	transactions, err := table.WithContext(ctx).
		Scopes(scopesToApply...).
		Where(p.conditionsBySpec(spec)...).
		Find()
	if err != nil {
		return nil, fmt.Errorf("failed to find known transactions: %w", err)
	}

	return slices.Map(transactions, mapModelToEntityKnownTx), nil
}

func (p *KnownTx) CountKnownTxs(ctx context.Context, spec *pkgentity.KnownTxReadSpecification, opts ...queryopts.Options) (int64, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-KnownTx-CountKnownTxs")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	table := &p.query.KnownTx

	count, err := table.WithContext(ctx).
		Scopes(scopes.FromQueryOptsForGen(table, opts)...).
		Where(p.conditionsBySpec(spec)...).
		Count()
	if err != nil {
		return 0, fmt.Errorf("failed to count known transactions: %w", err)
	}

	return count, nil
}

func (p *KnownTx) SetBatchForKnownTxs(ctx context.Context, txIDs []string, batch string) error {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-KnownTx-SetBatchForKnownTxs", attribute.StringSlice("TxIDs", txIDs), attribute.String("Batch", batch))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	if len(txIDs) == 0 {
		return nil
	}

	err = p.db.WithContext(ctx).Model(&models.KnownTx{}).
		Where("tx_id IN ? ", txIDs).
		UpdateColumn("batch", batch).Error
	if err != nil {
		return fmt.Errorf("failed to set batch for known transactions: %w", err)
	}
	return nil
}

func (p *KnownTx) conditionsBySpec(spec *pkgentity.KnownTxReadSpecification) []gen.Condition {
	if spec == nil {
		return nil
	}

	table := &p.query.KnownTx
	if spec.TxID != nil {
		return []gen.Condition{table.TxID.Eq(*spec.TxID)}
	}
	if len(spec.TxIDs) > 0 {
		return []gen.Condition{table.TxID.In(spec.TxIDs...)}
	}

	var conditions []gen.Condition
	if spec.Attempts != nil {
		conditions = append(conditions, cmpCondition(table.Attempts, spec.Attempts))
	}
	if spec.Status != nil {
		conditions = append(conditions, cmpCondition(table.Status, spec.Status.ToStringComparable()))
	}
	if spec.Notified != nil {
		conditions = append(conditions, cmpBoolCondition(table.Notified, spec.Notified))
	}
	if spec.BlockHeight != nil {
		conditions = append(conditions, cmpCondition(table.BlockHeight, spec.BlockHeight))
	}
	if spec.MerkleRoot != nil {
		conditions = append(conditions, cmpCondition(table.MerkleRoot, spec.MerkleRoot))
	}
	if spec.BlockHash != nil {
		conditions = append(conditions, cmpCondition(table.BlockHash, spec.BlockHash))
	}

	return conditions
}

// InvalidateMerkleProofsByBlockHash sets MerklePath, BlockHeight, MerkleRoot, and BlockHash
// to NULL for all KnownTx records where BlockHash matches any of the provided hashes.
// Also sets status to 'reorg' so CheckForProofsTask will re-fetch proofs.
// Adds a history note to each affected transaction.
// Returns the number of affected records.
func (p *KnownTx) InvalidateMerkleProofsByBlockHash(ctx context.Context, blockHashes []string) (int64, error) {
	var err error

	ctx, span := tracing.StartTracing(ctx, "Repository-KnownTx-InvalidMerkleProofsByClockHash",
		attribute.Int("block_hashes_count", len(blockHashes)))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	if len(blockHashes) == 0 {
		return 0, nil
	}

	var affected int64

	err = runInTransaction(ctx, p.db, p.retry, func(tx *gorm.DB) error {
		var affectedTxs []struct {
			TxID      string
			BlockHash string
		}

		if err = tx.Model(&models.KnownTx{}).
			Select("tx_id", "block_hash").
			Where("block_hash IN ?", blockHashes).
			Find(&affectedTxs).Error; err != nil {
			return fmt.Errorf("failed to find affected transactions: %w", err)
		}
		if len(affectedTxs) == 0 {
			return nil
		}

		res := tx.Model(&models.KnownTx{}).
			Where("block_hash IN ?", blockHashes).
			Updates(map[string]any{
				"merkle_path":   nil,
				"block_height":  nil,
				"merkle_root":   nil,
				"block_hash":    nil,
				"attempts":      0,
				"was_broadcast": true,
				"status":        wdk.ProvenTxStatusReorg,
			})
		if res.Error != nil {
			err = res.Error
			return fmt.Errorf("failed to invalidate merkle proofs: %w", err)
		}

		affected = res.RowsAffected

		// add history notes about reorg
		notes := make([]*pkgentity.TxHistoryNote, 0, len(affectedTxs))
		for _, tx := range affectedTxs {
			note := history.NewBuilder().
				ReorgInvalidatedProof(tx.BlockHash).
				Entity(tx.TxID)
			notes = append(notes, note)
		}

		if notesErr := addTxNotes(tx, notes); notesErr != nil {
			return fmt.Errorf("failed to add reorg history notes: %w", notesErr)
		}

		return nil
	})

	return affected, err
}

func mapModelToEntityKnownTx(model *models.KnownTx) *pkgentity.KnownTx {
	if model == nil {
		return nil
	}

	knownTx := &pkgentity.KnownTx{
		CreatedAt:           model.CreatedAt,
		UpdatedAt:           model.UpdatedAt,
		TxID:                model.TxID,
		Status:              model.Status,
		Attempts:            model.Attempts,
		Notified:            model.Notified,
		WasBroadcast:        model.WasBroadcast || model.Status.WasBroadcastStatus(),
		RebroadcastAttempts: model.RebroadcastAttempts,
		RawTx:               model.RawTx,
		InputBEEF:           model.InputBeef,
		BlockHeight:         model.BlockHeight,
		MerklePath:          model.MerklePath,
		MerkleRoot:          model.MerkleRoot,
		BlockHash:           model.BlockHash,
	}

	if model.TxNotes != nil {
		knownTx.TxNotes = slices.Map(model.TxNotes, mapModelToEntityTxNote)
	}

	return knownTx
}
