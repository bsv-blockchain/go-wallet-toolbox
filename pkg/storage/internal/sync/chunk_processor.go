package sync

import (
	"context"
	"fmt"
	"github.com/go-softwarelab/common/pkg/optional"
	"time"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/to"
)

type operation struct {
	updates int
	inserts int
}

var (
	singleUpdate = operation{updates: 1}
	singleInsert = operation{inserts: 1}
)

type chunkProcessor struct {
	parent    *processSyncChunk
	chunk     *wdk.SyncChunk
	result    wdk.ProcessSyncChunkResult
	ctx       context.Context
	user      *entity.User
	args      *wdk.RequestSyncChunkArgs
	syncState *entity.SyncState
}

func newChunkProcessor(ctx context.Context, parent *processSyncChunk, chunk *wdk.SyncChunk, args *wdk.RequestSyncChunkArgs, user *entity.User) *chunkProcessor {
	return &chunkProcessor{
		ctx:    ctx,
		parent: parent,
		chunk:  chunk,
		args:   args,
		user:   user,
	}
}

func (p *chunkProcessor) process() (err error) {
	syncState, err := p.parent.repo.FindSyncState(p.ctx, p.user.ID, p.args.FromStorageIdentityKey)
	if err != nil {
		return fmt.Errorf("failed to find sync state: %w", err)
	}

	if syncState == nil {
		return fmt.Errorf("sync state not found for userID %d and storage %s", p.user.ID, p.args.FromStorageIdentityKey)
	}

	p.syncState = syncState

	if p.chunk.User != nil {
		if err = p.mergeUser(); err != nil {
			return fmt.Errorf("failed to merge user: %w", err)
		}
	}

	if p.emptyChunk() {
		p.result.Done = true
		return nil // No data to process, return early.
	}

	for _, basket := range p.chunk.OutputBaskets {
		if err = p.upsertBaskets(basket); err != nil {
			return err
		}
	}

	for _, provenTxReq := range p.chunk.ProvenTxReqs {
		if err = p.upsertProvenTxReqs(provenTxReq); err != nil {
			return err
		}
	}

	for _, provenTx := range p.chunk.ProvenTxs {
		if err = p.upsertProvenTx(provenTx); err != nil {
			return err
		}
	}

	for _, transaction := range p.chunk.Transactions {
		if err = p.upsertTransaction(transaction); err != nil {
			return err
		}
	}

	err = p.parent.repo.UpdateSyncState(p.ctx, p.syncState)
	if err != nil {
		return fmt.Errorf("failed to update sync state: %w", err)
	}

	return nil
}

func (p *chunkProcessor) mergeUser() error {
	if p.chunk.User.IdentityKey != p.user.IdentityKey {
		return fmt.Errorf("chunk user identity key %s does not match current user identity key %s", p.chunk.User.IdentityKey, p.user.IdentityKey)
	}

	currentDBHasOlderVersion := p.user.UpdatedAt.Before(p.chunk.User.UpdatedAt)
	if !currentDBHasOlderVersion {
		return nil // No update needed
	}

	err := p.parent.repo.UpdateUser(p.ctx, p.user.ID, p.chunk.User.ActiveStorage, p.chunk.User.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to update user %d: %w", p.chunk.User.UserID, err)
	}

	p.updateResult(p.chunk.User.UpdatedAt, singleUpdate)
	return nil
}

func (p *chunkProcessor) upsertBaskets(chunkBasket *wdk.TableOutputBasket) error {
	// TODO: Upsert with UpdatedAt from chunkBasket.
	isNew, err := p.parent.repo.UpsertOutputBasket(p.ctx, p.user.ID, chunkBasket.BasketConfiguration)
	if err != nil {
		return fmt.Errorf("failed to upsert output basket %q: %w", chunkBasket.Name, err)
	}

	// NOTE: Even if the chunkBasket has exactly the same data as in the database, we still consider it an update.
	p.updateResult(chunkBasket.UpdatedAt, to.IfThen(isNew, singleInsert).ElseThen(singleUpdate))
	p.updateSyncState(wdk.OutputBasketEntityName, 1)

	// TODO: Most probably, we need to update the sync state (with IDMap) - But I postpone this until it is actually needed.

	return nil
}

func (p *chunkProcessor) upsertProvenTxReqs(chunkProvenTxReq *wdk.TableProvenTxReq) error {
	isNew, err := p.parent.repo.UpsertKnownTxForSync(p.ctx, &entity.KnownTx{
		CreatedAt: chunkProvenTxReq.CreatedAt,
		UpdatedAt: chunkProvenTxReq.UpdatedAt,
		TxID:      chunkProvenTxReq.TxID,
		Status:    chunkProvenTxReq.Status,
		Attempts:  chunkProvenTxReq.Attempts,
		Notified:  chunkProvenTxReq.Notified,
		RawTx:     chunkProvenTxReq.RawTx,
		InputBEEF: chunkProvenTxReq.InputBEEF,
	})
	if err != nil {
		return fmt.Errorf("failed to upsert proven tx req for TxID %q: %w", chunkProvenTxReq.TxID, err)
	}

	p.updateResult(chunkProvenTxReq.UpdatedAt, to.IfThen(isNew, singleInsert).ElseThen(singleUpdate))
	p.updateSyncState(wdk.ProvenTxReqEntityName, 1)

	return nil
}

func (p *chunkProcessor) upsertProvenTx(chunkProvenTx *wdk.TableProvenTx) error {
	isNew, err := p.parent.repo.UpsertKnownTxForSync(p.ctx, &entity.KnownTx{
		CreatedAt:   chunkProvenTx.CreatedAt,
		UpdatedAt:   chunkProvenTx.UpdatedAt,
		TxID:        chunkProvenTx.TxID,
		Status:      wdk.ProvenTxStatusCompleted,
		RawTx:       chunkProvenTx.RawTx,
		BlockHeight: to.Ptr(chunkProvenTx.Height),
		MerklePath:  chunkProvenTx.MerklePath,
		MerkleRoot:  to.Ptr(chunkProvenTx.MerkleRoot),
		BlockHash:   to.Ptr(chunkProvenTx.BlockHash),
	})
	if err != nil {
		return fmt.Errorf("failed to upsert proven tx for TxID %q: %w", chunkProvenTx.TxID, err)
	}

	p.updateResult(chunkProvenTx.UpdatedAt, to.IfThen(isNew, singleInsert).ElseThen(singleUpdate))
	p.updateSyncState(wdk.ProvenTxEntityName, 1)

	return nil
}

func (p *chunkProcessor) upsertTransaction(chunkTransaction *wdk.TableTransaction) error {
	if p.chunk.User != nil && p.chunk.User.UserID != chunkTransaction.UserID {
		return fmt.Errorf("chunk transaction user ID %d does not match chunk user ID %d", chunkTransaction.UserID, p.chunk.User.UserID)
	}

	isNew, transactionID, err := p.parent.repo.UpsertTransactionForSync(p.ctx, &entity.Transaction{
		CreatedAt:   chunkTransaction.CreatedAt,
		UpdatedAt:   chunkTransaction.UpdatedAt,
		UserID:      p.user.ID,
		Status:      chunkTransaction.Status,
		Reference:   string(chunkTransaction.Reference),
		IsOutgoing:  chunkTransaction.IsOutgoing,
		Satoshis:    chunkTransaction.Satoshis,
		Description: chunkTransaction.Description,
		Version:     optional.OfPtr(chunkTransaction.Version).OrZeroValue(),
		LockTime:    optional.OfPtr(chunkTransaction.LockTime).OrZeroValue(),
		TxID:        chunkTransaction.TxID,
		InputBEEF:   chunkTransaction.InputBEEF,
	})
	if err != nil {
		return fmt.Errorf("failed to upsert transaction for reference %q: %w", chunkTransaction.Reference, err)
	}

	_ = transactionID
	// TODO: Most probably, we need to store new TransactionID in the sync state (IDMap) - But I postpone this until it is actually needed.

	p.updateResult(chunkTransaction.UpdatedAt, to.IfThen(isNew, singleInsert).ElseThen(singleUpdate))
	p.updateSyncState(wdk.TransactionEntityName, 1)

	return nil
}

func (p *chunkProcessor) updateResult(updatedAt time.Time, operations ...operation) {
	for _, op := range operations {
		p.result.Updates += op.updates
		p.result.Inserts += op.inserts
	}

	if p.result.MaxUpdatedAt == nil || updatedAt.After(*p.result.MaxUpdatedAt) {
		p.result.MaxUpdatedAt = &updatedAt
	}
}

func (p *chunkProcessor) updateSyncState(entityName wdk.EntityName, count uint64) {
	syncMapEntity, exists := p.syncState.SyncMap[entityName]
	if !exists {
		syncMapEntity = wdk.NewSyncMapEntity(entityName)
		p.syncState.SyncMap[entityName] = syncMapEntity
	}

	syncMapEntity.Count += count
}

// emptyChunk checks if the chunk is empty, meaning it has no row data to process.
// NOTE: The user pointer is not taken into account.
func (p *chunkProcessor) emptyChunk() bool {
	// TODO: Add more entities when implemented.
	return len(p.chunk.OutputBaskets) == 0 &&
		len(p.chunk.ProvenTxs) == 0 &&
		len(p.chunk.ProvenTxReqs) == 0 &&
		len(p.chunk.Transactions) == 0
}
