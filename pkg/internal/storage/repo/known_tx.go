package repo

import (
	"context"
	"errors"
	"fmt"
	"iter"

	"github.com/bsv-blockchain/go-sdk/transaction"
	pkgentity "github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/genquery"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/scopes"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/history"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/slices"
	"gorm.io/gen"
	"gorm.io/gorm"
)

const (
	maxDepthOfRecursion = 1000
)

type KnownTx struct {
	db *gorm.DB
}

func NewKnownTxRepo(db *gorm.DB) *KnownTx {
	return &KnownTx{db: db}
}

func (p *KnownTx) UpsertKnownTx(ctx context.Context, req *entity.UpsertKnownTx, txNote history.Builder) error {
	err := p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return upsertKnownTx(tx, req, txNote)
	})

	if err != nil {
		return fmt.Errorf("failed to upsert known tx: %w", err)
	}
	return nil
}

func upsertKnownTx(tx *gorm.DB, req *entity.UpsertKnownTx, txNote history.Builder) error {
	var model models.KnownTx
	err := tx.First(&model, "tx_id = ? ", req.TxID).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("cannot upsert known tx: %w", err)
	}

	if req.SkipForStatus != nil && model.Status == *req.SkipForStatus {
		// If the status is the same as the one we want to skip, we do not update it.
		return nil
	}

	model.Status = req.Status
	model.TxID = req.TxID
	model.RawTx = req.RawTx
	model.InputBeef = req.InputBeef

	err = addTxNote(tx, txNote.Entity(req.TxID))
	if err != nil {
		return err
	}

	err = tx.Save(&model).Error
	if err != nil {
		return fmt.Errorf("cannot save known tx: %w", err)
	}

	return nil
}

func updateKnownTxStatus(tx *gorm.DB, txID string, status wdk.ProvenTxReqStatus, txNotes []history.Builder) error {
	var model models.KnownTx
	err := tx.Model(&model).
		Where("tx_id = ? ", txID).
		UpdateColumn("status", status).
		Error
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

func (p *KnownTx) FindKnownTxRawTx(ctx context.Context, txID string) ([]byte, error) {
	var model models.KnownTx
	err := p.db.WithContext(ctx).
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
	if len(txIDs) == 0 {
		return make(map[string][]byte), nil
	}

	var results []struct {
		TxID  string
		RawTx []byte
	}

	err := p.db.WithContext(ctx).
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

func (p *KnownTx) FindKnownTxStatus(ctx context.Context, txID string) (wdk.ProvenTxReqStatus, error) {
	var model models.KnownTx
	err := p.db.WithContext(ctx).
		Model(&model).
		Select("status").
		Where("tx_id = ? ", txID).
		First(&model).Error
	if err != nil {
		return "", fmt.Errorf("failed to find proven tx status: %w", err)
	}
	return model.Status, nil
}

func (p *KnownTx) AllKnownTxsExist(ctx context.Context, txIDs []string, sourceTxsStatusFilter []wdk.ProvenTxReqStatus) (bool, error) {
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
	err := query.Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("failed to check if known transactions exist: %w", err)
	}

	return count == int64(len(txIDs)), nil
}

func (p *KnownTx) BuildValidBEEF(ctx context.Context, txID string, statusesToFilterOut []wdk.ProvenTxReqStatus) (*transaction.Beef, error) {
	beef := transaction.NewBeefV2()
	err := p.recursiveBuildValidBEEF(ctx, 0, beef, txID, statusesToFilterOut)
	if err != nil {
		return nil, fmt.Errorf("failed to build valid BEEF: %w", err)
	}

	return beef, nil
}

func (p *KnownTx) recursiveBuildValidBEEF(ctx context.Context, depth int, mergeToBeef *transaction.Beef, txID string, statusesToFilterOut []wdk.ProvenTxReqStatus) error {
	if depth > maxDepthOfRecursion {
		return fmt.Errorf("max depth of recursion reached: %d", maxDepthOfRecursion)
	}

	var model models.KnownTx
	query := p.db.WithContext(ctx).
		Model(&model).
		Select("raw_tx, input_beef, merkle_path")

	if len(statusesToFilterOut) > 0 {
		query = query.Where("status NOT IN ? ", statusesToFilterOut)
	}

	err := query.First(&model, "tx_id = ? ", txID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return fmt.Errorf("failed to find known tx, raw tx and input beef for tx (id: %s): %w", txID, err)
	}

	if model.RawTx == nil || model.InputBeef == nil {
		return fmt.Errorf("raw tx or input beef is nil in transaction %s", txID)
	}

	tx, err := transaction.NewTransactionFromBytes(model.RawTx)
	if err != nil {
		return fmt.Errorf("failed to build transaction object from raw tx (id: %s): %w", txID, err)
	}

	if model.HasMerklePath() {
		merklePath, err := transaction.NewMerklePathFromBinary(model.MerklePath)
		if err != nil {
			return fmt.Errorf("failed to build merkle path from binary for tx (id: %s): %w", txID, err)
		}
		err = tx.AddMerkleProof(merklePath)
		if err != nil {
			return fmt.Errorf("failed to add merkle proof to transaction (id: %s): %w", txID, err)
		}

		_, err = mergeToBeef.MergeTransaction(tx)
		if err != nil {
			return fmt.Errorf("failed to merge transaction (id: %s) into BEEF object: %w", txID, err)
		}

		return nil
	}

	for i := range tx.Inputs {
		if len(tx.Inputs[i].SourceTXID) == 0 {
			return fmt.Errorf("input of tx (id: %s) has empty SourceTXID at index %d ", txID, i)
		}
	}

	_, err = mergeToBeef.MergeRawTx(model.RawTx, nil)
	if err != nil {
		return fmt.Errorf("failed to merge raw tx (id: %s) into BEEF object: %w", txID, err)
	}

	err = mergeToBeef.MergeBeefBytes(model.InputBeef)
	if err != nil {
		return fmt.Errorf("failed to merge input beef into BEEF object: %w", err)
	}

	var sourceTXID string
	for _, input := range tx.Inputs {
		sourceTXID = input.SourceTXID.String()
		beefTx := mergeToBeef.Transactions[sourceTXID]
		if beefTx == nil || beefTx.DataFormat != transaction.RawTxAndBumpIndex {
			err = p.recursiveBuildValidBEEF(ctx, depth+1, mergeToBeef, sourceTXID, statusesToFilterOut)
			if err != nil {
				return fmt.Errorf("failed to recursively find known tx and merge into BEEF: %w", err)
			}
		}
	}

	// Result is in mergeToBeef
	return nil
}

func (p *KnownTx) GetBEEFForTxIDs(ctx context.Context, txids iter.Seq[string], knownTxIDs []string, statusesToFilterOut []wdk.ProvenTxReqStatus) ([]byte, error) {
	beef := transaction.NewBeefV2()

	// TODO: handle KnownTxids properly which works in a way that for provided KnownTxids beef will do `MergeTxIDOnly` instead of recursively fetching parent transactions
	_ = knownTxIDs

	for txid := range txids {
		if beef.FindTransaction(txid) != nil {
			continue
		}
		err := p.recursiveBuildValidBEEF(ctx, 0, beef, txid, statusesToFilterOut)
		if err != nil {
			return nil, fmt.Errorf("failed for txid %s: %w", txid, err)
		}
	}

	data, err := beef.Bytes()
	if err != nil {
		return nil, fmt.Errorf("beef serialization error: %w", err)
	}

	return data, nil
}

func (p *KnownTx) FindKnownTxIDsByStatuses(ctx context.Context, limit int, txStatus ...wdk.ProvenTxReqStatus) ([]*entity.KnownTxForStatusSync, error) {
	var rows []*models.KnownTx
	err := p.db.WithContext(ctx).
		Model(&models.KnownTx{}).
		Select("tx_id, status, attempts").
		Where("status IN ? ", txStatus).
		Order("created_at ASC").
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find known tx ids by statuses: %w", err)
	}

	return slices.Map(rows, func(row *models.KnownTx) *entity.KnownTxForStatusSync {
		return &entity.KnownTxForStatusSync{
			TxID:     row.TxID,
			Attempts: row.Attempts,
			Status:   row.Status,
		}
	}), nil
}

func (p *KnownTx) UpdateKnownTxAsMined(ctx context.Context, knownTxAsMined *entity.KnownTxAsMined) error {
	err := p.db.WithContext(ctx).Model(&models.KnownTx{}).
		Where("tx_id = ?", knownTxAsMined.TxID).
		Updates(&models.KnownTx{
			Status:      wdk.ProvenTxStatusCompleted,
			BlockHash:   &knownTxAsMined.BlockHash,
			BlockHeight: &knownTxAsMined.BlockHeight,
			MerklePath:  knownTxAsMined.MerklePath,
			MerkleRoot:  &knownTxAsMined.MerkleRoot,
			Notified:    true,
		}).Error
	if err != nil {
		return fmt.Errorf("failed to update known tx as mined: %w", err)
	}
	return nil
}

func (p *KnownTx) IncreaseKnownTxAttemptsForTxIDs(ctx context.Context, txIDs []string) error {
	if len(txIDs) == 0 {
		return nil
	}

	err := p.db.WithContext(ctx).Model(&models.KnownTx{}).
		Where("tx_id IN ? ", txIDs).
		UpdateColumn("attempts", gorm.Expr("attempts + 1")).Error
	if err != nil {
		return fmt.Errorf("failed to increase attempts for tx ids: %w", err)
	}
	return nil
}

func (p *KnownTx) SetStatusForKnownTxsAboveAttempts(ctx context.Context, attempts uint64, status wdk.ProvenTxReqStatus) error {
	if attempts == 0 {
		return nil
	}

	err := p.db.WithContext(ctx).Model(&models.KnownTx{}).
		Where("attempts >= ? ", attempts).
		UpdateColumn("status", status).Error
	if err != nil {
		return fmt.Errorf("failed to set status for known transactions above attempts: %w", err)
	}
	return nil
}

func (p *KnownTx) FindKnownTxs(ctx context.Context, spec *pkgentity.KnownTxReadSpecification, opts ...queryopts.Options) ([]*pkgentity.KnownTx, error) {
	table := genquery.KnownTx

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
	table := genquery.KnownTx

	count, err := table.WithContext(ctx).
		Scopes(scopes.FromQueryOptsForGen(table, opts)...).
		Where(p.conditionsBySpec(spec)...).
		Count()
	if err != nil {
		return 0, fmt.Errorf("failed to count known transactions: %w", err)
	}

	return count, nil
}

func (p *KnownTx) conditionsBySpec(spec *pkgentity.KnownTxReadSpecification) []gen.Condition {
	if spec == nil {
		return []gen.Condition{}
	}

	if spec.TxID != nil {
		return []gen.Condition{genquery.KnownTx.TxID.Eq(*spec.TxID)}
	}

	var conditions []gen.Condition

	// TODO: Add more conditions based on the spec

	return conditions
}

func mapModelToEntityKnownTx(model *models.KnownTx) *pkgentity.KnownTx {
	if model == nil {
		return nil
	}

	knownTx := &pkgentity.KnownTx{
		CreatedAt:   model.CreatedAt,
		UpdatedAt:   model.UpdatedAt,
		TxID:        model.TxID,
		Status:      model.Status,
		Attempts:    model.Attempts,
		Notified:    model.Notified,
		RawTx:       model.RawTx,
		InputBEEF:   model.InputBeef,
		BlockHeight: model.BlockHeight,
		MerklePath:  model.MerklePath,
		MerkleRoot:  model.MerkleRoot,
		BlockHash:   model.BlockHash,
	}

	if model.TxNotes != nil {
		knownTx.TxNotes = slices.Map(model.TxNotes, mapModelToEntityTxNote)
	}

	return knownTx
}
