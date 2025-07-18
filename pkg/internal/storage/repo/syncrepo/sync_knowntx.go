package syncrepo

import (
	"context"
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/genquery"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/scopes"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"gorm.io/gorm"
)

type SyncKnownTx struct {
	db *gorm.DB
}

func NewSyncKnownTx(db *gorm.DB) *SyncKnownTx {
	return &SyncKnownTx{db: db}
}

type KnownTxWithNum struct {
	models.KnownTx
	NumID int
}

func (s *SyncKnownTx) tableName() string {
	return genquery.KnownTx.TableName()
}

func (s *SyncKnownTx) FindKnownTxsForSync(ctx context.Context, userID int, opts ...queryopts.Options) ([]*wdk.TableProvenTxReq, []*wdk.TableProvenTx, error) {
	filters := append(scopes.FromQueryOpts(opts), s.whereExistsScope(userID))

	resultModels, err := findChunk[models.KnownTx, KnownTxWithNum](ctx, s.db, s.tableName(), "tx_id", filters)
	if err != nil {
		return nil, nil, fmt.Errorf("find known transactions for sync: %w", err)
	}

	provenTxReqs, provenTxs := s.toReqOrProvenTx(resultModels)
	return provenTxReqs, provenTxs, nil
}

func (s *SyncKnownTx) UpsertKnownTxForSync(ctx context.Context, entity *entity.KnownTx) (isNew bool, err error) {
	model := models.KnownTx{
		CreatedAt:   entity.CreatedAt,
		UpdatedAt:   entity.UpdatedAt,
		TxID:        entity.TxID,
		Status:      entity.Status,
		Attempts:    entity.Attempts,
		Notified:    entity.Notified,
		RawTx:       entity.RawTx,
		InputBeef:   entity.InputBEEF,
		BlockHeight: entity.BlockHeight,
		MerklePath:  entity.MerklePath,
		MerkleRoot:  entity.MerkleRoot,
		BlockHash:   entity.BlockHash,
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updateTx := tx.Model(&models.KnownTx{}).
			Where("tx_id = ?", entity.TxID).
			Updates(model)

		if updateTx.Error != nil {
			return fmt.Errorf("failed to update proven tx req: %w", updateTx.Error)
		}

		if updateTx.RowsAffected > 0 {
			return nil
		}

		err := tx.Create(&model).Error
		if err != nil {
			return fmt.Errorf("failed to create proven tx req: %w", err)
		}

		isNew = true

		return nil
	})

	if err != nil {
		return false, fmt.Errorf("transaction failed: %w", err)
	}

	return isNew, nil
}

func (s *SyncKnownTx) whereExistsScope(userID int) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		whereExistClause := fmt.Sprintf(
			"EXISTS (SELECT 1 FROM %s as user_tx WHERE user_tx.tx_id = %s.tx_id AND user_tx.user_id = ?)",
			genquery.Transaction.TableName(),
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
func (s *SyncKnownTx) toReqOrProvenTx(models []*KnownTxWithNum) ([]*wdk.TableProvenTxReq, []*wdk.TableProvenTx) {
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
			provenTxReqs = append(provenTxReqs, s.mapModelToTableProvenTxReqForSync(model))
		}
	}

	return provenTxReqs, provenTxs
}

func (s *SyncKnownTx) mapModelToTableProvenTxReqForSync(model *KnownTxWithNum) *wdk.TableProvenTxReq {
	if model.MerklePath != nil {
		// this mapping function is designed to convert a model that is guaranteed to be NOT MINED (does not have a Merkle path),
		panic("KnownTx model must not have MerklePath set when creating TableProvenTxReq for sync")
	}

	return &wdk.TableProvenTxReq{
		CreatedAt:     model.CreatedAt,
		UpdatedAt:     model.UpdatedAt,
		ProvenTxReqID: model.NumID,
		Status:        model.Status,
		Attempts:      model.Attempts,
		Notified:      model.Notified,
		TxID:          model.TxID,
		Batch:         nil,  // TODO: For now batch broadcasting is not supported, will be added later
		History:       "{}", // TODO: History feature will be reworked later, then we can address this and think if we even want to sync "history" field
		Notify:        "{}", // TODO: Notify includes transaction IDs and they are only used by JS-version of the wallet, so we can ignore it for now
		RawTx:         model.RawTx,
		InputBEEF:     model.InputBeef,
	}
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
