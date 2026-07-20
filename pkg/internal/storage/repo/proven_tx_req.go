package repo

import (
	"context"
	"encoding/json"
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
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/history"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/tracing"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

const (
	maxDepthOfRecursion = 1000
)

type ProvenTxReqRepo struct {
	db    *gorm.DB
	query *genquery.Query
}

func NewProvenTxReqRepo(db *gorm.DB, query *genquery.Query) *ProvenTxReqRepo {
	return &ProvenTxReqRepo{db: db, query: query}
}

func (p *ProvenTxReqRepo) UpsertProvenTxReq(ctx context.Context, req *entity.UpsertProvenTxReq, txNote history.Builder) error {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-KnownTx-UpsertProvenTxReq", attribute.String("TxID", req.TxID))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	err = p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return upsertProvenTxReq(tx, req, txNote)
	})
	if err != nil {
		return fmt.Errorf("failed to upsert known tx: %w", err)
	}
	return nil
}

func (p *ProvenTxReqRepo) UpdateKnownTxStatus(ctx context.Context, txID string, status wdk.ProvenTxReqStatus, skipForStatuses []wdk.ProvenTxReqStatus, txNotes []history.Builder) error {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-KnownTx-UpdateKnownTxStatus", attribute.String("TxID", txID), attribute.String("Status", string(status)))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	return updateKnownTxStatus(p.db.WithContext(ctx), txID, status, skipForStatuses, txNotes)
}

func (p *ProvenTxReqRepo) MarkKnownTxsAsSubmitting(ctx context.Context, txIDs []string) error {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-KnownTx-MarkKnownTxsAsSubmitting")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	if len(txIDs) == 0 {
		return nil
	}

	err = p.db.WithContext(ctx).
		Model(&models.ProvenTxReq{}).
		Where("txid IN ?", txIDs).
		Where("status = ?", wdk.ProvenTxStatusUnprocessed).
		UpdateColumns(map[string]any{
			"status":       wdk.ProvenTxStatusSending,
			"wasBroadcast": true,
		}).Error
	if err != nil {
		return fmt.Errorf("failed to mark known txs as submitting: %w", err)
	}

	return nil
}

func upsertProvenTxReq(tx *gorm.DB, req *entity.UpsertProvenTxReq, txNote history.Builder) error {
	var model models.ProvenTxReq
	err := tx.First(&model, "txid = ? ", req.TxID).Error
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

func updateKnownTxStatus(tx *gorm.DB, txID string, status wdk.ProvenTxReqStatus, skipForStatuses []wdk.ProvenTxReqStatus, txNotes []history.Builder) error {
	var model models.ProvenTxReq

	query := tx.Model(&model).Where("txid = ? ", txID)
	if len(skipForStatuses) > 0 {
		query = query.Where("status NOT IN ? ", skipForStatuses)
	}

	updates := map[string]any{
		"status": status,
	}
	if status.WasBroadcastStatus() {
		updates["wasBroadcast"] = true
	}

	err := query.UpdateColumns(updates).Error
	if err != nil {
		return fmt.Errorf("failed to update known tx status: %w", err)
	}

	err = addTxNotes(tx, slices.Map(txNotes, func(note history.Builder) *pkgentity.TxHistoryNote {
		return note.Entity(txID)
	}))
	if err != nil {
		return err
	}

	return nil
}

func (p *ProvenTxReqRepo) FindKnownTxRawTx(ctx context.Context, txID string) ([]byte, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-KnownTx-FindKnownTxRawTx", attribute.String("TxID", txID))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	var model models.ProvenTxReq
	err = p.db.WithContext(ctx).
		Model(&model).
		Select("rawTx").
		First(&model, "txid = ? ", txID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find raw tx of known tx: %w", err)
	}
	return model.RawTx, nil
}

func (p *ProvenTxReqRepo) FindKnownTxRawTxs(ctx context.Context, txIDs []string) (map[string][]byte, error) {
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
		Model(&models.ProvenTxReq{}).
		Select("txid, rawTx").
		Where("txid IN ?", txIDs).
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

func (p *ProvenTxReqRepo) FindKnownTxStatuses(ctx context.Context, txIDs ...string) (map[string]wdk.ProvenTxReqStatus, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-KnownTx-FindKnownTxStatuses", attribute.StringSlice("TxIDs", txIDs))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	var rows []*models.ProvenTxReq
	err = p.db.WithContext(ctx).
		Model(&models.ProvenTxReq{}).
		Select("status, txid").
		Where("txid IN (?)", txIDs).
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find proven tx statuses for list of txIDs: %w", err)
	}

	txIDStatuses := seq.MapTo(seq.FromSlice(rows), func(row *models.ProvenTxReq) (string, wdk.ProvenTxReqStatus) {
		return row.TxID, row.Status
	})

	return seq2.CollectToMap(txIDStatuses), nil
}

func (p *ProvenTxReqRepo) AllKnownTxsExist(ctx context.Context, txIDs []string, sourceTxsStatusFilter []wdk.ProvenTxReqStatus) (bool, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-KnownTx-AllKnownTxsExist", attribute.StringSlice("TxIDs", txIDs))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	var model models.ProvenTxReq
	query := p.db.WithContext(ctx).
		Model(&model).
		Select("txid").
		Where("txid IN (?) ", txIDs).
		Where("rawTx IS NOT NULL").
		Where("LENGTH(rawTx) > 0").
		Where("inputBEEF IS NOT NULL").
		Where("LENGTH(inputBEEF) > 0")

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

func (p *ProvenTxReqRepo) FindKnownTxIDsByStatuses(ctx context.Context, txStatus []wdk.ProvenTxReqStatus, opts ...queryopts.Options) ([]*entity.ProvenTxReqForStatusSync, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-KnownTx-FindKnownTxIDsByStatuses")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	var rows []*models.ProvenTxReq
	err = p.db.WithContext(ctx).
		Model(&models.ProvenTxReq{}).
		Select("txid, status, attempts, wasBroadcast, rebroadcastAttempts, batch").
		Scopes(scopes.FromQueryOpts(opts)...).
		Where("status IN ? ", txStatus).
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find known tx ids by statuses: %w", err)
	}

	return mapKnownTxRowsForStatusSync(rows), nil
}

func (p *ProvenTxReqRepo) FindKnownTxIDsReadyForStatusSync(ctx context.Context, txStatus []wdk.ProvenTxReqStatus, opts ...queryopts.Options) ([]*entity.ProvenTxReqForStatusSync, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-KnownTx-FindKnownTxIDsReadyForStatusSync")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	var rows []*models.ProvenTxReq
	query := p.db.WithContext(ctx).
		Model(&models.ProvenTxReq{}).
		Select("txid, status, attempts, wasBroadcast, rebroadcastAttempts, batch").
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
		return query.Where("status = ? AND wasBroadcast = ?", wdk.ProvenTxStatusUnsent, true)
	}

	return query.Where(
		"(status IN ? OR (status = ? AND wasBroadcast = ?))",
		statusesWithoutUnsent,
		wdk.ProvenTxStatusUnsent,
		true,
	)
}

func (p *ProvenTxReqRepo) FindKnownTxIDsByStatusesNeedingFailureReview(ctx context.Context, txStatus []wdk.ProvenTxReqStatus, limit int) ([]*entity.ProvenTxReqForStatusSync, error) {
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

	knownTxTable := p.query.ProvenTxReq.TableName()
	transactionTable := p.query.Transaction.TableName()
	outputTable := p.query.Output.TableName()

	var rows []*models.ProvenTxReq
	err = p.db.WithContext(ctx).
		Model(&models.ProvenTxReq{}).
		Select("txid, status, attempts, wasBroadcast, rebroadcastAttempts, batch").
		Where("status IN ? ", txStatus).
		Where(fmt.Sprintf(`
			EXISTS (
				SELECT 1
				FROM %s
				LEFT JOIN %s
					ON %s.spentBy = %s.transactionId
				WHERE %s.txid = %s.txid
					AND (%s.status <> ? OR %s.outputId IS NOT NULL)
			)
		`, transactionTable, outputTable, outputTable, transactionTable, transactionTable, knownTxTable, transactionTable, outputTable), wdk.TxStatusFailed).
		Order(fmt.Sprintf("%s.created_at ASC", knownTxTable)).
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find failed known tx ids needing review: %w", err)
	}

	return mapKnownTxRowsForStatusSync(rows), nil
}

func mapKnownTxRowsForStatusSync(rows []*models.ProvenTxReq) []*entity.ProvenTxReqForStatusSync {
	return slices.Map(rows, func(row *models.ProvenTxReq) *entity.ProvenTxReqForStatusSync {
		return &entity.ProvenTxReqForStatusSync{
			TxID:                row.TxID,
			Attempts:            row.Attempts,
			RebroadcastAttempts: uint64(row.RebroadcastAttempts),
			Status:              row.Status,
			WasBroadcast:        row.WasBroadcast || row.Status.WasBroadcastStatus(),
			Batch:               row.Batch,
		}
	})
}

func (p *ProvenTxReqRepo) UpdateKnownTxAsMined(ctx context.Context, knownTxAsMined *entity.ProvenTxAsMined) error {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-KnownTx-UpdateKnownTxAsMined", attribute.String("TxID", knownTxAsMined.TxID))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	err = p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var req models.ProvenTxReq
		if err := tx.Select("rawTx").Where("txid = ?", knownTxAsMined.TxID).First(&req).Error; err != nil {
			return fmt.Errorf("failed to get rawTx from bsv_proven_tx_reqs: %w", err)
		}

		provenTx := models.ProvenTx{
			TxID:       knownTxAsMined.TxID,
			Height:     &knownTxAsMined.BlockHeight,
			MerklePath: knownTxAsMined.MerklePath,
			RawTx:      req.RawTx,
			BlockHash:  &knownTxAsMined.BlockHash,
			MerkleRoot: &knownTxAsMined.MerkleRoot,
		}

		// If we can extract index from MerklePath, we would do it here. For now we leave it 0 or nil depending on DB constraint
		var defaultIndex uint64 = 0
		provenTx.Index = &defaultIndex

		if err := tx.Create(&provenTx).Error; err != nil {
			return fmt.Errorf("failed to insert proven_txs: %w", err)
		}

		err = tx.Model(&models.ProvenTxReq{}).
			Where("txid = ?", knownTxAsMined.TxID).
			Updates(map[string]any{
				"provenTxId":   provenTx.ProvenTxID,
				"status":       wdk.ProvenTxStatusCompleted,
				"wasBroadcast": true,
				"notified":     true,
			}).Error
		if err != nil {
			return fmt.Errorf("failed to update proven_tx_req: %w", err)
		}

		err = addTxNotes(tx, slices.Map(knownTxAsMined.Notes, func(note history.Builder) *pkgentity.TxHistoryNote {
			return note.Entity(knownTxAsMined.TxID)
		}))
		if err != nil {
			return fmt.Errorf("failed to add tx notes: %w", err)
		}

		// NOTE: There can be multiple transactions with the same txid, so we need to update all of them.
		err = tx.Model(&models.Transaction{}).
			Where(p.query.Transaction.TxID.Eq(knownTxAsMined.TxID)).
			Updates(map[string]any{
				p.query.Transaction.Status.ColumnName().String(): wdk.TxStatusCompleted,
			}).Error
		if err != nil {
			return fmt.Errorf("failed to update transaction status as completed: %w", err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("db transaction failed: %w", err)
	}
	return nil
}

func (p *ProvenTxReqRepo) IncreaseKnownTxAttemptsForTxIDs(ctx context.Context, txIDs []string) error {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-KnownTx-IncreaseKnownTxAttemptsForTxIDs", attribute.StringSlice("TxIDs", txIDs))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	if len(txIDs) == 0 {
		return nil
	}

	err = p.db.WithContext(ctx).Model(&models.ProvenTxReq{}).
		Where("txid IN ? ", txIDs).
		UpdateColumn("attempts", gorm.Expr("attempts + 1")).Error
	if err != nil {
		return fmt.Errorf("failed to increase attempts for tx ids: %w", err)
	}
	return nil
}

func (p *ProvenTxReqRepo) ApplyProofTimeouts(ctx context.Context, attempts, maxRebroadcastAttempts uint64, statuses []wdk.ProvenTxReqStatus) ([]models.ProvenTxReq, error) {
	var (
		err        error
		updatedTxs []models.ProvenTxReq
	)
	ctx, span := tracing.StartTracing(ctx, "Repository-KnownTx-ApplyProofTimeouts", attribute.String("Attempts", fmt.Sprintf("%d", attempts)))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	if attempts == 0 {
		return nil, nil
	}

	err = p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var timedOut []*models.ProvenTxReq
		query := tx.Model(&models.ProvenTxReq{}).
			Where("attempts >= ?", attempts)
		if len(statuses) > 0 {
			query = withReadyForStatusSyncFilter(query, statuses)
		}

		if findErr := query.
			Select("txid, status, attempts, wasBroadcast, rebroadcastAttempts").
			Find(&timedOut).Error; findErr != nil {
			return fmt.Errorf("failed to find known transactions above attempts: %w", findErr)
		}

		updatedTxs = make([]models.ProvenTxReq, 0, len(timedOut))
		for _, knownTx := range timedOut {
			updates := proofTimeoutUpdates(knownTx, maxRebroadcastAttempts)
			if updateErr := tx.Model(&models.ProvenTxReq{}).
				Where("txid = ?", knownTx.TxID).
				UpdateColumns(updates).Error; updateErr != nil {
				return fmt.Errorf("failed to apply proof timeout for known transaction %s: %w", knownTx.TxID, updateErr)
			}

			knownTx.Status = updates["status"].(wdk.ProvenTxReqStatus)
			knownTx.Attempts = updates["attempts"].(uint64)
			knownTx.WasBroadcast = updates["wasBroadcast"].(bool)
			knownTx.RebroadcastAttempts = updates["rebroadcastAttempts"].(int)
			updatedTxs = append(updatedTxs, *knownTx)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to apply proof timeouts: %w", err)
	}
	return updatedTxs, nil
}

func proofTimeoutUpdates(knownTx *models.ProvenTxReq, maxRebroadcastAttempts uint64) map[string]any {
	wasBroadcast := knownTx.WasBroadcast || knownTx.Status.WasBroadcastStatus()
	if wasBroadcast && (maxRebroadcastAttempts == 0 || knownTx.RebroadcastAttempts < int(maxRebroadcastAttempts)) {
		return map[string]any{
			"status":              wdk.ProvenTxStatusUnsent,
			"attempts":            uint64(0),
			"wasBroadcast":        true,
			"rebroadcastAttempts": knownTx.RebroadcastAttempts + 1,
		}
	}

	return map[string]any{
		"status":              wdk.ProvenTxStatusInvalid,
		"attempts":            knownTx.Attempts,
		"wasBroadcast":        wasBroadcast,
		"rebroadcastAttempts": knownTx.RebroadcastAttempts,
	}
}

func (p *ProvenTxReqRepo) FindKnownTxs(ctx context.Context, spec *pkgentity.ProvenTxReqReadSpecification, opts ...queryopts.Options) ([]*pkgentity.ProvenTxReq, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-KnownTx-FindKnownTxs")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	table := &p.query.ProvenTxReq

	scopesToApply := append(scopes.FromQueryOptsForGen(table, opts))

	transactions, err := table.WithContext(ctx).
		Scopes(scopesToApply...).
		Where(p.conditionsBySpec(spec)...).
		Find()
	if err != nil {
		return nil, fmt.Errorf("failed to find known transactions: %w", err)
	}

	return slices.Map(transactions, mapModelToEntityKnownTx), nil
}

func (p *ProvenTxReqRepo) CountKnownTxs(ctx context.Context, spec *pkgentity.ProvenTxReqReadSpecification, opts ...queryopts.Options) (int64, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-KnownTx-CountKnownTxs")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	table := &p.query.ProvenTxReq

	count, err := table.WithContext(ctx).
		Scopes(scopes.FromQueryOptsForGen(table, opts)...).
		Where(p.conditionsBySpec(spec)...).
		Count()
	if err != nil {
		return 0, fmt.Errorf("failed to count known transactions: %w", err)
	}

	return count, nil
}

func (p *ProvenTxReqRepo) SetBatchForKnownTxs(ctx context.Context, txIDs []string, batch string) error {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-KnownTx-SetBatchForKnownTxs", attribute.StringSlice("TxIDs", txIDs), attribute.String("Batch", batch))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	if len(txIDs) == 0 {
		return nil
	}

	err = p.db.WithContext(ctx).Model(&models.ProvenTxReq{}).
		Where("txid IN ? ", txIDs).
		UpdateColumn("batch", batch).Error
	if err != nil {
		return fmt.Errorf("failed to set batch for known transactions: %w", err)
	}
	return nil
}

func (p *ProvenTxReqRepo) conditionsBySpec(spec *pkgentity.ProvenTxReqReadSpecification) []gen.Condition {
	if spec == nil {
		return nil
	}

	table := &p.query.ProvenTxReq
	if spec.TxID != nil {
		return []gen.Condition{table.TxID.Eq(*spec.TxID)}
	}
	if len(spec.TxIDs) > 0 {
		return []gen.Condition{table.TxID.In(spec.TxIDs...)}
	}

	var conditions []gen.Condition
	if spec.Attempts != nil {
		conditions = append(conditions, cmpCondition(table.Attempts, mapComparableUint32ToUint64(spec.Attempts)))
	}
	if spec.Status != nil {
		conditions = append(conditions, cmpCondition(table.Status, spec.Status.ToStringComparable()))
	}
	if spec.Notified != nil {
		conditions = append(conditions, cmpBoolCondition(table.Notified, spec.Notified))
	}

	return conditions
}

// InvalidateMerkleProofsByBlockHash sets MerklePath, BlockHeight, MerkleRoot, and BlockHash
// to NULL for all KnownTx records where BlockHash matches any of the provided hashes.
// Also sets status to 'reorg' so CheckForProofsTask will re-fetch proofs.
// Adds a history note to each affected transaction.
// Returns the number of affected records.
func (p *ProvenTxReqRepo) InvalidateMerkleProofsByBlockHash(ctx context.Context, blockHashes []string) (int64, error) {
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

	err = p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var affectedTxs []struct {
			TxID      string
			BlockHash string
		}

		if err = tx.Model(&models.ProvenTxReq{}).
			Select("txid", "blockHash").
			Where("blockHash IN ?", blockHashes).
			Find(&affectedTxs).Error; err != nil {
			return fmt.Errorf("failed to find affected transactions: %w", err)
		}
		if len(affectedTxs) == 0 {
			return nil
		}

		res := tx.Model(&models.ProvenTxReq{}).
			Where("blockHash IN ?", blockHashes).
			Updates(map[string]any{
				"merkle_path":  nil,
				"block_height": nil,
				"merkle_root":  nil,
				"blockHash":   nil,
				"attempts":     0,
				"wasBroadcast": true,
				"status":       wdk.ProvenTxStatusReorg,
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

		if err := addTxNotes(tx, notes); err != nil {
			return fmt.Errorf("failed to add reorg history notes: %w", err)
		}

		return nil
	})

	return affected, nil
}

func mapModelToEntityKnownTx(model *models.ProvenTxReq) *pkgentity.ProvenTxReq {
	if model == nil {
		return nil
	}

	knownTx := &pkgentity.ProvenTxReq{
		CreatedAt:           model.CreatedAt,
		UpdatedAt:           model.UpdatedAt,
		TxID:                model.TxID,
		Status:              model.Status,
		Attempts:            uint32(model.Attempts),
		Notified:            model.Notified,
		WasBroadcast:        model.WasBroadcast || model.Status.WasBroadcastStatus(),
		RebroadcastAttempts: uint32(model.RebroadcastAttempts),
		RawTx:               model.RawTx,
		InputBEEF:           model.InputBeef,
	}

	if model.History != "" {
		_ = json.Unmarshal([]byte(model.History), &knownTx.History)
	}

	return knownTx
}

func mapComparableUint32ToUint64(c *pkgentity.Comparable[uint32]) *pkgentity.Comparable[uint64] {
	if c == nil {
		return nil
	}
	var inValues []uint64
	if c.InValues != nil {
		for _, v := range c.InValues {
			inValues = append(inValues, uint64(v))
		}
	}
	return &pkgentity.Comparable[uint64]{
		Value:      uint64(c.Value),
		ValueRight: uint64(c.ValueRight),
		InValues:   inValues,
		Cmp:        c.Cmp,
	}
}
