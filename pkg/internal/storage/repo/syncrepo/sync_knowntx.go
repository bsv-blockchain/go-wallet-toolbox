package syncrepo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/go-softwarelab/common/pkg/slices"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/genquery"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/scopes"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type SyncKnownTx struct {
	db    *gorm.DB
	query *genquery.Query
}

func NewSyncKnownTx(db *gorm.DB, query *genquery.Query) *SyncKnownTx {
	return &SyncKnownTx{db: db, query: query}
}

type KnownTxWithNum struct {
	models.KnownTx

	NumID int
}

func (s *SyncKnownTx) tableName() string {
	return s.query.KnownTx.TableName()
}

func (s *SyncKnownTx) FindKnownTxsForSync(ctx context.Context, userID int, opts ...queryopts.Options) ([]*wdk.TableProvenTxReq, []*wdk.TableProvenTx, error) {
	filters := append(scopes.FromQueryOpts(opts), s.whereExistsScope(userID))

	var resultModels []*KnownTxWithNum

	var model models.KnownTx

	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := upsertNumericIDLookup(ctx, s.db, tx, s.query, func(db *gorm.DB) *gorm.DB {
			return db.
				Select(fmt.Sprintf("?, %s", "tx_id"), s.tableName()).
				Scopes(filters...).
				Find(&model)
		}); err != nil {
			return fmt.Errorf("failed to upsert numeric ID lookup: %w", err)
		}

		if err := tx.WithContext(ctx).
			Model(&model).
			Select("*").
			Scopes(joinWithNumericIDLookupScope(s.query, "tx_id", s.tableName(), clause.InnerJoin)).
			Scopes(filters...).
			// Deterministic tiebreak: tx_id is this table's unique primary key,
			// appended after the Paginate scope's non-unique `created_at DESC` so
			// offset pagination is stable across engines. See FindOutputsForSync.
			Scopes(func(db *gorm.DB) *gorm.DB {
				return db.Order(clause.OrderByColumn{Column: clause.Column{Table: s.tableName(), Name: "tx_id"}})
			}).
			Preload(s.query.KnownTx.TxNotes.Name()).
			Find(&resultModels).Error; err != nil {
			return fmt.Errorf("failed to find proven tx requests for sync: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("transaction failed: %w", err)
	}

	provenTxReqs, provenTxs, err := s.toReqOrProvenTx(resultModels)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to convert models to proven tx requests and proven transactions: %w", err)
	}

	return provenTxReqs, provenTxs, nil
}

func (s *SyncKnownTx) UpsertKnownTxForSync(ctx context.Context, entity *entity.KnownTx) (isNew bool, knownTxNumID uint, err error) {
	model := models.KnownTx{
		CreatedAt:           entity.CreatedAt,
		UpdatedAt:           entity.UpdatedAt,
		TxID:                entity.TxID,
		Status:              entity.Status,
		Attempts:            entity.Attempts,
		WasBroadcast:        entity.WasBroadcast || entity.Status.WasBroadcastStatus(),
		RebroadcastAttempts: entity.RebroadcastAttempts,
		Notified:            entity.Notified,
		RawTx:               entity.RawTx,
		InputBeef:           entity.InputBEEF,
		BlockHeight:         entity.BlockHeight,
		MerklePath:          entity.MerklePath,
		MerkleRoot:          entity.MerkleRoot,
		BlockHash:           entity.BlockHash,
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Always ensure a numeric_id_lookup entry exists so processSyncChunk can
		// populate provenTx/provenTxReq idMap (readerID → writerID) for round-trips.
		// Mint before the BRC-40 guard so stale/equal skips still contribute idMap entries.
		numID, numErr := s.saveNumericIDForKnownTx(ctx, tx, entity.TxID)
		if numErr != nil {
			return numErr
		}
		knownTxNumID = numID

		// BRC-40 stale-chunk guard: only apply UPDATE when incoming is strictly newer.
		// Equal updated_at must SKIP. TxNote delete/insert side-effects only fire
		// inside the post-guard branch. See ts-stack EntityProvenTx.mergeExisting and
		// conformance vector sync.brc40.merge.proventx.error.regression.1.
		var existing models.KnownTx
		existsErr := tx.Model(&models.KnownTx{}).
			Select("tx_id, updated_at").
			Where("tx_id = ?", entity.TxID).
			First(&existing).Error

		if existsErr == nil {
			if !model.UpdatedAt.After(existing.UpdatedAt) {
				return nil
			}

			updateTx := tx.Model(&models.KnownTx{}).
				Where("tx_id = ? AND updated_at < ?", entity.TxID, model.UpdatedAt).
				Updates(model)

			if updateTx.Error != nil {
				return fmt.Errorf("failed to update proven tx req: %w", updateTx.Error)
			}

			if updateTx.RowsAffected == 0 {
				return nil
			}

			if deleteErr := tx.Delete(&models.TxNote{}, "tx_id = ?", entity.TxID).Error; deleteErr != nil {
				return fmt.Errorf("failed to delete existing transaction notes: %w", deleteErr)
			}

			if noteErr := s.addHistoryNotes(ctx, tx, entity.TxID, entity.TxNotes); noteErr != nil {
				return fmt.Errorf("failed to add transaction history notes while updating knownTx: %w", noteErr)
			}

			return nil
		}

		if !errors.Is(existsErr, gorm.ErrRecordNotFound) {
			return fmt.Errorf("failed to lookup existing known tx: %w", existsErr)
		}

		if err = tx.Create(&model).Error; err != nil {
			return fmt.Errorf("failed to create proven tx req: %w", err)
		}

		if err = s.addHistoryNotes(ctx, tx, entity.TxID, entity.TxNotes); err != nil {
			return fmt.Errorf("failed to add transaction history notes while creating knownTx: %w", err)
		}

		isNew = true

		return nil
	})
	if err != nil {
		return false, 0, fmt.Errorf("transaction failed: %w", err)
	}

	return isNew, knownTxNumID, nil
}

func (s *SyncKnownTx) saveNumericIDForKnownTx(ctx context.Context, tx *gorm.DB, txID string) (uint, error) {
	if err := saveNumericIDLookup(ctx, tx, s.tableName(), txID); err != nil {
		return 0, fmt.Errorf("failed to save numeric ID lookup for known tx %q: %w", txID, err)
	}
	return findNumericIDLookup(ctx, tx, s.tableName(), txID)
}

func (s *SyncKnownTx) whereExistsScope(userID int) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		whereExistClause := fmt.Sprintf(
			"EXISTS (SELECT 1 FROM %s as user_tx WHERE user_tx.tx_id = %s.tx_id AND user_tx.user_id = ?)",
			s.query.Transaction.TableName(),
			s.tableName(),
		)

		return db.Where(whereExistClause, userID)
	}
}

// toReqOrProvenTx produced two slices:
// - one for requests (ProvenTxReq) that do not have a Merkle path (not mined transactions)
// - one for proven transactions (ProvenTx) that have a Merkle path (mined transactions).
// NOTE: In this implementation, there is ONLY ONE table to hold both: requests (ProvenTxReq) and proven transactions (ProvenTx).
// The function is used to prepare data for syncing, where we need to distinguish between requests and proven transactions.
func (s *SyncKnownTx) toReqOrProvenTx(models []*KnownTxWithNum) ([]*wdk.TableProvenTxReq, []*wdk.TableProvenTx, error) {
	minedTxs := 0
	for _, model := range models {
		if model.HasMerklePath() {
			minedTxs++
		}
	}

	provenTxReqs := make([]*wdk.TableProvenTxReq, 0, len(models)-minedTxs)
	provenTxs := make([]*wdk.TableProvenTx, 0, minedTxs)

	for _, model := range models {
		if model.HasMerklePath() {
			provenTxs = append(provenTxs, s.mapModelToTableProvenTxForSync(model))
		} else {
			provenTxReq, err := s.mapModelToTableProvenTxReqForSync(model)
			if err != nil {
				return nil, nil, err
			}
			provenTxReqs = append(provenTxReqs, provenTxReq)
		}
	}

	return provenTxReqs, provenTxs, nil
}

func (s *SyncKnownTx) mapModelToTableProvenTxReqForSync(model *KnownTxWithNum) (*wdk.TableProvenTxReq, error) {
	if model.MerklePath != nil {
		// this mapping function is designed to convert a model that is guaranteed to be NOT MINED (does not have a Merkle path),
		panic("KnownTx model must not have MerklePath set when creating TableProvenTxReq for sync")
	}

	historyNotes, err := s.mapModelToHistoryNotes(model)
	if err != nil {
		return nil, err
	}

	return &wdk.TableProvenTxReq{
		CreatedAt:           model.CreatedAt,
		UpdatedAt:           model.UpdatedAt,
		ProvenTxReqID:       model.NumID,
		Status:              model.Status,
		Attempts:            model.Attempts,
		WasBroadcast:        model.WasBroadcast || model.Status.WasBroadcastStatus(),
		RebroadcastAttempts: model.RebroadcastAttempts,
		Notified:            model.Notified,
		TxID:                model.TxID,
		Batch:               nil, // TODO: For now batch broadcasting is not supported, will be added later
		History:             historyNotes,
		Notify:              "{}", // TODO: Notify includes transaction IDs and they are only used by JS-version of the wallet, so we can ignore it for now
		RawTx:               model.RawTx,
		InputBEEF:           model.InputBeef,
	}, nil
}

func (s *SyncKnownTx) mapModelToTableProvenTxForSync(model *KnownTxWithNum) *wdk.TableProvenTx {
	if model.MerklePath == nil || model.BlockHeight == nil || model.MerkleRoot == nil || model.BlockHash == nil {
		// this mapping function is designed to convert a model that is guaranteed to be MINED (has a Merkle path),
		// this should never happen, but if it does, we panic to indicate a programming error
		panic("KnownTx model must have MerklePath, BlockHeight, MerkleRoot, and BlockHash set when creating TableProvenTx for sync")
	}

	return &wdk.TableProvenTx{
		CreatedAt:  model.CreatedAt,
		UpdatedAt:  model.UpdatedAt,
		ProvenTxID: model.NumID,
		TxID:       model.TxID,
		Height:     *model.BlockHeight,
		Index:      0, // TODO: JS version also contains an index, it could be done in separate task later
		MerklePath: model.MerklePath,
		RawTx:      model.RawTx,
		BlockHash:  *model.BlockHash,
		MerkleRoot: *model.MerkleRoot,
	}
}

func (s *SyncKnownTx) mapModelToHistoryNotes(model *KnownTxWithNum) (string, error) {
	if len(model.TxNotes) == 0 {
		return "{}", nil
	}

	notes := slices.Map(model.TxNotes, func(it *models.TxNote) *wdk.HistoryNote {
		return &wdk.HistoryNote{
			When:       it.CreatedAt,
			UserID:     it.UserID,
			What:       it.What,
			Attributes: it.Attributes,
		}
	})

	notesObj := struct {
		Notes []*wdk.HistoryNote `json:"notes"`
	}{
		Notes: notes,
	}

	historyNotes, err := json.Marshal(notesObj)
	if err != nil {
		return "", fmt.Errorf("failed to marshal transaction notes: %w", err)
	}

	return string(historyNotes), nil
}

func (s *SyncKnownTx) addHistoryNotes(ctx context.Context, tx *gorm.DB, txID string, notes []*entity.TxHistoryNote) error {
	if len(notes) == 0 {
		return nil
	}

	modelsToAdd := slices.Map(notes, func(note *entity.TxHistoryNote) *models.TxNote {
		return &models.TxNote{
			CreatedAt:  note.When,
			TxID:       txID,
			UserID:     note.UserID,
			What:       note.What,
			Attributes: note.Attributes,
		}
	})

	if err := tx.WithContext(ctx).Create(&modelsToAdd).Error; err != nil {
		return fmt.Errorf("failed to create transaction history notes: %w", err)
	}

	return nil
}
