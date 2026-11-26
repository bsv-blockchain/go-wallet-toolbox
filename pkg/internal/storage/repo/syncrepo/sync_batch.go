package syncrepo

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// The Upsert*sForSync methods are the slice-taking counterparts of the Upsert*ForSync methods. They
// keep the same BRC-40 merge rules (create when missing, update only when strictly newer, skip when
// stale or equal) and return one result per input, in input order.
//
// With the connection's InsertBatchSize at 0 (the default) they simply run the single-row method
// for every element. With a batch size they run in ONE database transaction, look the existing rows
// up in bulk and insert the missing ones with multi-row INSERTs of at most that many rows. Inputs
// that repeat a key (which must see each other's effects in order) always use the single-row path.

// insertBatchSize is the configured rows per INSERT; 0 means single-row behaviour.
func insertBatchSize(db *gorm.DB) int {
	return db.CreateBatchSize
}

// createRows inserts rows with multi-row INSERTs of at most the connection's batch size.
func createRows[T any](tx *gorm.DB, rows []*T, clauses ...clause.Expression) error {
	if len(rows) == 0 {
		return nil
	}

	size := insertBatchSize(tx)
	if size <= 0 {
		size = len(rows)
	}

	if err := tx.Clauses(clauses...).CreateInBatches(rows, size).Error; err != nil {
		return fmt.Errorf("batched insert of %d rows failed: %w", len(rows), err)
	}

	return nil
}

// forChunks calls fn with consecutive slices of at most size elements (all of them if size <= 0).
func forChunks[T any](items []T, size int, fn func(chunk []T) error) error {
	if size <= 0 || size > len(items) {
		size = len(items)
	}
	for start := 0; start < len(items); start += size {
		if err := fn(items[start:min(start+size, len(items))]); err != nil {
			return err
		}
	}

	return nil
}

// KnownTxSyncResult is the outcome of one UpsertKnownTxsForSync element.
type KnownTxSyncResult struct {
	IsNew        bool
	KnownTxNumID uint
}

// UpsertKnownTxsForSync is the batched UpsertKnownTxForSync.
func (s *SyncKnownTx) UpsertKnownTxsForSync(ctx context.Context, entities []*entity.KnownTx) ([]KnownTxSyncResult, error) {
	results := make([]KnownTxSyncResult, len(entities))
	if len(entities) == 0 {
		return results, nil
	}

	if insertBatchSize(s.db) <= 0 || hasDuplicates(entities, func(e *entity.KnownTx) string { return e.TxID }) {
		for i, e := range entities {
			isNew, numID, err := s.UpsertKnownTxForSync(ctx, e)
			if err != nil {
				return nil, err
			}
			results[i] = KnownTxSyncResult{IsNew: isNew, KnownTxNumID: numID}
		}

		return results, nil
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return s.upsertKnownTxsInTx(ctx, tx, entities, results)
	})
	if err != nil {
		return nil, fmt.Errorf("transaction failed: %w", err)
	}

	return results, nil
}

func (s *SyncKnownTx) upsertKnownTxsInTx(ctx context.Context, tx *gorm.DB, entities []*entity.KnownTx, results []KnownTxSyncResult) error {
	size := insertBatchSize(tx)
	txIDs := make([]string, len(entities))
	lookups := make([]*models.NumericIDLookup, len(entities))
	for i, e := range entities {
		txIDs[i] = e.TxID
		lookups[i] = &models.NumericIDLookup{TableName: s.tableName(), StringID: e.TxID}
	}

	// numeric ids first, even for rows that end up skipped (see UpsertKnownTxForSync)
	if err := createRows(tx, lookups, clause.OnConflict{DoNothing: true}); err != nil {
		return fmt.Errorf("failed to save numeric ID lookups: %w", err)
	}
	numIDs := make(map[string]uint, len(entities))
	err := forChunks(txIDs, size, func(chunk []string) error {
		var found []models.NumericIDLookup
		if findErr := tx.Where("table_name = ? AND string_id IN ?", s.tableName(), chunk).Find(&found).Error; findErr != nil {
			return fmt.Errorf("failed to find numeric IDs: %w", findErr)
		}
		for _, f := range found {
			numIDs[f.StringID] = uint(f.NumID) //nolint:gosec // autoincrement ids are positive
		}

		return nil
	})
	if err != nil {
		return err
	}

	existing := make(map[string]models.KnownTx, len(entities))
	err = forChunks(txIDs, size, func(chunk []string) error {
		var found []models.KnownTx
		if findErr := tx.Select("tx_id, updated_at").Where("tx_id IN ?", chunk).Find(&found).Error; findErr != nil {
			return fmt.Errorf("failed to lookup existing known txs: %w", findErr)
		}
		for _, f := range found {
			existing[f.TxID] = f
		}

		return nil
	})
	if err != nil {
		return err
	}

	var (
		newModels []*models.KnownTx
		newNotes  []*models.TxNote
	)
	for i, e := range entities {
		numID, ok := numIDs[e.TxID]
		if !ok {
			return fmt.Errorf("numeric ID not found for %q", e.TxID)
		}
		results[i].KnownTxNumID = numID

		model := knownTxModelFromEntity(e)
		if current, found := existing[e.TxID]; found {
			if err = s.updateKnownTxIfNewer(ctx, tx, e, model, current); err != nil {
				return err
			}
			continue
		}

		results[i].IsNew = true
		newModels = append(newModels, &model)
		for _, note := range e.TxNotes {
			newNotes = append(newNotes, &models.TxNote{
				CreatedAt:  note.When,
				TxID:       e.TxID,
				UserID:     note.UserID,
				What:       note.What,
				Attributes: note.Attributes,
			})
		}
	}

	if err = createRows(tx, newModels); err != nil {
		return fmt.Errorf("failed to create proven tx reqs: %w", err)
	}
	if err = createRows(tx, newNotes); err != nil {
		return fmt.Errorf("failed to add transaction history notes while creating knownTxs: %w", err)
	}

	return nil
}

// TransactionSyncResult is the outcome of one UpsertTransactionsForSync element.
type TransactionSyncResult struct {
	IsNew         bool
	TransactionID uint
}

// UpsertTransactionsForSync is the batched UpsertTransactionForSync.
func (s *SyncTransaction) UpsertTransactionsForSync(ctx context.Context, entities []*entity.Transaction) ([]TransactionSyncResult, error) {
	results := make([]TransactionSyncResult, len(entities))
	if len(entities) == 0 {
		return results, nil
	}

	type key struct {
		userID    int
		reference string
	}
	if insertBatchSize(s.db) <= 0 || hasDuplicates(entities, func(e *entity.Transaction) key { return key{e.UserID, e.Reference} }) {
		for i, e := range entities {
			isNew, id, err := s.UpsertTransactionForSync(ctx, e)
			if err != nil {
				return nil, err
			}
			results[i] = TransactionSyncResult{IsNew: isNew, TransactionID: id}
		}

		return results, nil
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		size := insertBatchSize(tx)

		refsByUser := map[int][]string{}
		for _, e := range entities {
			refsByUser[e.UserID] = append(refsByUser[e.UserID], e.Reference)
		}
		existing := make(map[key]models.Transaction, len(entities))
		for userID, refs := range refsByUser {
			err := forChunks(refs, size, func(chunk []string) error {
				var found []models.Transaction
				if findErr := tx.Select("id, user_id, reference, updated_at").
					Where("user_id = ? AND reference IN ?", userID, chunk).Find(&found).Error; findErr != nil {
					return fmt.Errorf("failed to lookup existing transactions: %w", findErr)
				}
				for _, f := range found {
					existing[key{f.UserID, f.Reference}] = f
				}

				return nil
			})
			if err != nil {
				return err
			}
		}

		var newModels []*models.Transaction
		newIndex := make([]int, 0, len(entities))
		for i, e := range entities {
			model := transactionModelFromEntity(e)
			current, found := existing[key{e.UserID, e.Reference}]
			if !found {
				newModels = append(newModels, &model)
				newIndex = append(newIndex, i)
				continue
			}

			results[i].TransactionID = current.ID
			if !model.UpdatedAt.After(current.UpdatedAt) {
				continue
			}
			// RowsAffected == 0 means a concurrent writer advanced updated_at: treat as skip.
			if err := tx.Model(&models.Transaction{}).
				Where("id = ? AND updated_at < ?", current.ID, model.UpdatedAt).
				Updates(model).Error; err != nil {
				return fmt.Errorf("failed to update transaction: %w", err)
			}
		}

		if err := createRows(tx, newModels); err != nil {
			return fmt.Errorf("failed to create transactions: %w", err)
		}
		for j, model := range newModels {
			if model.ID == 0 {
				return errors.New("transaction ID is zero after creation, this should not happen")
			}
			results[newIndex[j]] = TransactionSyncResult{IsNew: true, TransactionID: model.ID}
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("transaction failed: %w", err)
	}

	return results, nil
}

// OutputSyncResult is the outcome of one UpsertOutputsForSync element.
type OutputSyncResult struct {
	IsNew    bool
	OutputID uint
}

// UpsertOutputsForSync is the batched UpsertOutputForSync, including the user UTXO upsert.
func (s *SyncOutput) UpsertOutputsForSync(ctx context.Context, entities []*entity.Output) ([]OutputSyncResult, error) {
	results := make([]OutputSyncResult, len(entities))
	if len(entities) == 0 {
		return results, nil
	}

	if insertBatchSize(s.db) <= 0 || hasDuplicates(entities, outputKeyOf) {
		for i, e := range entities {
			isNew, id, err := s.UpsertOutputForSync(ctx, e)
			if err != nil {
				return nil, err
			}
			results[i] = OutputSyncResult{IsNew: isNew, OutputID: id}
		}

		return results, nil
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return s.upsertOutputsInTx(tx, entities, results)
	})
	if err != nil {
		return nil, fmt.Errorf("transaction failed: %w", err)
	}

	return results, nil
}

// outputKey is the natural key of an output: (user, transaction, vout).
type outputKey struct {
	userID        int
	transactionID uint
	vout          uint32
}

func outputKeyOf(e *entity.Output) outputKey { return outputKey{e.UserID, e.TransactionID, e.Vout} }

func (s *SyncOutput) upsertOutputsInTx(tx *gorm.DB, entities []*entity.Output, results []OutputSyncResult) error {
	size := insertBatchSize(tx)

	txIDs := make([]uint, 0, len(entities))
	seenTx := map[uint]struct{}{}
	for _, e := range entities {
		if _, ok := seenTx[e.TransactionID]; !ok {
			seenTx[e.TransactionID] = struct{}{}
			txIDs = append(txIDs, e.TransactionID)
		}
	}
	statuses := make(map[uint]wdk.TxStatus, len(txIDs))
	err := forChunks(txIDs, size, func(chunk []uint) error {
		var found []models.Transaction
		if findErr := tx.Select("id, status").Where("id IN ?", chunk).Find(&found).Error; findErr != nil {
			return fmt.Errorf("failed to check known transactions: %w", findErr)
		}
		for _, f := range found {
			statuses[f.ID] = f.Status
		}

		return nil
	})
	if err != nil {
		return err
	}

	tuples := make([][]any, len(entities))
	for i, e := range entities {
		if _, ok := statuses[e.TransactionID]; !ok {
			return fmt.Errorf("failed to check known transaction: %w", gorm.ErrRecordNotFound)
		}
		tuples[i] = []any{e.UserID, e.TransactionID, e.Vout}
	}
	existing := make(map[outputKey]models.Output, len(entities))
	err = forChunks(tuples, size, func(chunk [][]any) error {
		var found []models.Output
		if findErr := tx.Select("id, user_id, transaction_id, vout, updated_at").
			Where("(user_id, transaction_id, vout) IN ?", chunk).Find(&found).Error; findErr != nil {
			return fmt.Errorf("failed to lookup existing outputs: %w", findErr)
		}
		for _, f := range found {
			existing[outputKey{f.UserID, f.TransactionID, f.Vout}] = f
		}

		return nil
	})
	if err != nil {
		return err
	}

	var (
		newModels []*models.Output
		newIndex  []int
	)
	for i, e := range entities {
		model := outputModelFromEntity(e)
		current, found := existing[outputKeyOf(e)]
		if !found {
			newModels = append(newModels, &model)
			newIndex = append(newIndex, i)
			continue
		}

		results[i].OutputID = current.ID
		if !model.UpdatedAt.After(current.UpdatedAt) {
			continue
		}
		// Select("*") so zero values (e.g. BasketName of relinquished outputs) are written too.
		if err = tx.Model(&model).
			Where("id = ? AND updated_at < ?", current.ID, model.UpdatedAt).
			Select("*").
			Updates(&model).Error; err != nil {
			return fmt.Errorf("failed to update output: %w", err)
		}
	}

	if err = createRows(tx, newModels); err != nil {
		return fmt.Errorf("failed to create outputs: %w", err)
	}
	for j, model := range newModels {
		if model.ID == 0 {
			return errors.New("output ID is zero after creation, this should not happen")
		}
		results[newIndex[j]] = OutputSyncResult{IsNew: true, OutputID: model.ID}
	}

	return s.upsertUserUTXOsInTx(tx, entities, results, statuses)
}

// upsertUserUTXOsInTx applies the user UTXOs of the entities: update the existing row, collect the
// ones that need creating and insert those in batches.
func (s *SyncOutput) upsertUserUTXOsInTx(tx *gorm.DB, entities []*entity.Output, results []OutputSyncResult, statuses map[uint]wdk.TxStatus) error {
	var toCreate []*models.UserUTXO
	for i, e := range entities {
		utxoStatus := s.utxoStatusByTxStatus(statuses[e.TransactionID])
		if e.UserUTXO == nil || utxoStatus == wdk.UTXOStatusUnknown {
			continue
		}
		e.UserUTXO.OutputID = results[i].OutputID
		e.UserUTXO.Status = utxoStatus

		model := &models.UserUTXO{
			UserID:             e.UserUTXO.UserID,
			OutputID:           e.UserUTXO.OutputID,
			BasketName:         e.UserUTXO.BasketName,
			Satoshis:           e.UserUTXO.Satoshis,
			EstimatedInputSize: e.UserUTXO.EstimatedInputSize,
			CreatedAt:          e.UserUTXO.CreatedAt,
			ReservedByID:       e.UserUTXO.ReservedByID,
			UTXOStatus:         e.UserUTXO.Status,
		}
		updated := tx.Model(&models.UserUTXO{}).
			Where("user_id = ? AND output_id = ?", e.UserUTXO.UserID, e.UserUTXO.OutputID).
			Select("*").
			Updates(model)
		if updated.Error != nil {
			return fmt.Errorf("failed to update user UTXO: %w", updated.Error)
		}
		if updated.RowsAffected == 0 {
			toCreate = append(toCreate, model)
		}
	}

	if err := createRows(tx, toCreate); err != nil {
		return fmt.Errorf("failed to create user UTXOs: %w", err)
	}

	return nil
}

func hasDuplicates[T any, K comparable](items []T, keyOf func(T) K) bool {
	seen := make(map[K]struct{}, len(items))
	for _, item := range items {
		k := keyOf(item)
		if _, dup := seen[k]; dup {
			return true
		}
		seen[k] = struct{}{}
	}

	return false
}
