package sync

import (
	"context"
	"fmt"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/txutils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/optional"
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
	parent          *processSyncChunk
	chunk           *wdk.SyncChunk
	result          wdk.ProcessSyncChunkResult
	ctx             context.Context
	user            *entity.User
	args            *wdk.RequestSyncChunkArgs
	syncState       *entity.SyncState
	basketNameCache map[uint]string
	labelCache      map[uint]*entity.Label
}

func newChunkProcessor(ctx context.Context, parent *processSyncChunk, chunk *wdk.SyncChunk, args *wdk.RequestSyncChunkArgs, user *entity.User) *chunkProcessor {
	return &chunkProcessor{
		ctx:             ctx,
		parent:          parent,
		chunk:           chunk,
		args:            args,
		user:            user,
		basketNameCache: map[uint]string{},
		labelCache:      map[uint]*entity.Label{},
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
		err = p.updateSyncStateOnDone()
		if err != nil {
			return fmt.Errorf("failed to update sync state on done: %w", err)
		}

		p.result.MaxUpdatedAt = p.syncState.When
		p.result.Done = true

		return nil
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

	for _, output := range p.chunk.Outputs {
		if err = p.upsertOutput(output); err != nil {
			return fmt.Errorf("failed to upsert output: %w", err)
		}
	}

	for _, label := range p.chunk.TxLabels {
		if err = p.upsertLabel(label); err != nil {
			return fmt.Errorf("failed to upsert label: %w", err)
		}
	}

	for _, labelMap := range p.chunk.TxLabelMaps {
		if err = p.upsertLabelMap(labelMap); err != nil {
			return fmt.Errorf("failed to upsert label map: %w", err)
		}
	}

	err = p.parent.repo.UpdateSyncState(p.ctx, p.syncState)
	if err != nil {
		return fmt.Errorf("failed to update sync state: %w", err)
	}

	p.result.MaxUpdatedAt = p.syncState.SyncMap.MaxUpdatedAt()

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

	p.updateOperations(singleUpdate)
	return nil
}

func (p *chunkProcessor) upsertBaskets(chunkBasket *wdk.TableOutputBasket) error {
	if p.chunk.User != nil && p.chunk.User.UserID != chunkBasket.UserID {
		return fmt.Errorf("chunk basket user ID %d does not match chunk user ID %d", chunkBasket.UserID, p.chunk.User.UserID)
	}

	isNew, basketNumID, err := p.parent.repo.UpsertOutputBasketForSync(p.ctx, entity.OutputBasket{
		Name:                    string(chunkBasket.Name),
		UserID:                  p.user.ID,
		CreatedAt:               chunkBasket.CreatedAt,
		UpdatedAt:               chunkBasket.UpdatedAt,
		NumberOfDesiredUTXOs:    chunkBasket.NumberOfDesiredUTXOs,
		MinimumDesiredUTXOValue: chunkBasket.MinimumDesiredUTXOValue,
	})
	if err != nil {
		return fmt.Errorf("failed to upsert output basket %q: %w", chunkBasket.Name, err)
	}

	// NOTE: Even if the chunkBasket has exactly the same data as in the database, we still consider it an update.
	p.updateOperations(to.IfThen(isNew, singleInsert).ElseThen(singleUpdate))
	err = p.updateSyncState(wdk.OutputBasketEntityName, chunkBasket.UpdatedAt, 1, idDictionary{
		readerID: chunkBasket.BasketID,
		writerID: basketNumID,
	})
	if err != nil {
		return fmt.Errorf("failed to update sync state for output basket %q: %w", chunkBasket.Name, err)
	}

	p.basketNameCache[basketNumID] = string(chunkBasket.Name)

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

	p.updateOperations(to.IfThen(isNew, singleInsert).ElseThen(singleUpdate))
	err = p.updateSyncState(wdk.ProvenTxReqEntityName, chunkProvenTxReq.UpdatedAt, 1)
	if err != nil {
		return fmt.Errorf("failed to update sync state for proven tx req %q: %w", chunkProvenTxReq.TxID, err)
	}

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

	p.updateOperations(to.IfThen(isNew, singleInsert).ElseThen(singleUpdate))
	err = p.updateSyncState(wdk.ProvenTxEntityName, chunkProvenTx.UpdatedAt, 1)
	if err != nil {
		return fmt.Errorf("failed to update sync state for proven tx %q: %w", chunkProvenTx.TxID, err)
	}

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

	readerID, err := to.IntFromUnsigned(chunkTransaction.TransactionID)
	if err != nil {
		return fmt.Errorf("failed to convert transaction ID %d to int: %w", chunkTransaction.TransactionID, err)
	}

	p.updateOperations(to.IfThen(isNew, singleInsert).ElseThen(singleUpdate))
	err = p.updateSyncState(wdk.TransactionEntityName, chunkTransaction.UpdatedAt, 1, idDictionary{
		readerID: readerID,
		writerID: transactionID,
	})
	if err != nil {
		return fmt.Errorf("failed to update sync state for transaction with reference %q: %w", chunkTransaction.Reference, err)
	}

	return nil
}

func (p *chunkProcessor) upsertOutput(chunkOutput *wdk.TableOutput) error {
	if p.chunk.User != nil && p.chunk.User.UserID != chunkOutput.UserID {
		return fmt.Errorf("chunk output user ID %d does not match chunk user ID %d", chunkOutput.UserID, p.chunk.User.UserID)
	}

	var basketName *string
	if chunkOutput.BasketID != nil {
		basketIDOnWriterSide, err := p.translateID(wdk.OutputBasketEntityName, *chunkOutput.BasketID)
		if err != nil {
			return fmt.Errorf("failed to translate basket ID %d: %w", *chunkOutput.BasketID, err)
		}

		name, err := p.getBasketNameByNumID(basketIDOnWriterSide)
		if err != nil {
			return fmt.Errorf("failed to get basket name for basket ID %d: %w", basketIDOnWriterSide, err)
		}

		basketName = &name
	}

	transactionIDOnWriterSide, err := p.translateIDFromUnsigned(wdk.TransactionEntityName, chunkOutput.TransactionID)
	if err != nil {
		return fmt.Errorf("failed to translate transaction ID %d: %w", chunkOutput.TransactionID, err)
	}

	var spentByTransactionIDOnWriterSide *uint
	if chunkOutput.SpentBy != nil {
		spentByTransactionID, err := p.translateIDFromUnsigned(wdk.TransactionEntityName, *chunkOutput.SpentBy)
		if err != nil {
			return fmt.Errorf("failed to translate spent by transaction ID %d: %w", *chunkOutput.SpentBy, err)
		}
		spentByTransactionIDOnWriterSide = &spentByTransactionID
	}

	output := &entity.Output{
		CreatedAt:          chunkOutput.CreatedAt,
		UpdatedAt:          chunkOutput.UpdatedAt,
		UserID:             p.user.ID,
		TransactionID:      transactionIDOnWriterSide,
		SpentBy:            spentByTransactionIDOnWriterSide,
		Satoshis:           chunkOutput.Satoshis,
		TxID:               chunkOutput.TxID,
		Vout:               chunkOutput.Vout,
		LockingScript:      chunkOutput.LockingScript,
		CustomInstructions: chunkOutput.CustomInstructions,
		DerivationPrefix:   chunkOutput.DerivationPrefix,
		DerivationSuffix:   chunkOutput.DerivationSuffix,
		Spendable:          chunkOutput.Spendable,
		Change:             chunkOutput.Change,
		Description:        chunkOutput.OutputDescription,
		ProvidedBy:         chunkOutput.ProvidedBy,
		Purpose:            chunkOutput.Purpose,
		Type:               chunkOutput.Type,
		SenderIdentityKey:  chunkOutput.SenderIdentityKey,
		Tags:               nil, //TODO: Implement it along with tags backup support.
		BasketName:         basketName,
	}

	if chunkOutput.Spendable && basketName != nil && *basketName == wdk.BasketNameForChange {
		satoshis, err := to.UInt64(chunkOutput.Satoshis)
		if err != nil {
			return fmt.Errorf("failed to convert change-basket's satoshis %d to uint64: %w", chunkOutput.Satoshis, err)
		}

		output.UserUTXO = &entity.UserUTXO{
			UserID:             p.user.ID,
			BasketName:         wdk.BasketNameForChange,
			Satoshis:           satoshis,
			EstimatedInputSize: txutils.EstimatedInputSizeByType(wdk.OutputType(output.Type)),
			CreatedAt:          chunkOutput.CreatedAt,
			ReservedByID:       nil, //TODO: Talk to Damian how to deal with this - as it cannot be deduced from the output.
		}
	}

	isNew, _, err := p.parent.repo.UpsertOutputForSync(p.ctx, output)
	if err != nil {
		return fmt.Errorf("failed to upsert output for transaction ID %d, vout %d: %w", chunkOutput.TransactionID, chunkOutput.Vout, err)
	}

	p.updateOperations(to.IfThen(isNew, singleInsert).ElseThen(singleUpdate))
	err = p.updateSyncState(wdk.OutputEntityName, chunkOutput.UpdatedAt, 1)
	if err != nil {
		return fmt.Errorf("failed to update sync state for output with transaction ID %d and vout %d: %w", chunkOutput.TransactionID, chunkOutput.Vout, err)
	}

	return nil
}

func (p *chunkProcessor) upsertLabel(chunkLabel *wdk.TableTxLabel) error {
	if p.chunk.User != nil && p.chunk.User.UserID != chunkLabel.UserID {
		return fmt.Errorf("chunk label user ID %d does not match chunk user ID %d", chunkLabel.UserID, p.chunk.User.UserID)
	}

	entityLabel := &entity.Label{
		CreatedAt: chunkLabel.CreatedAt,
		UpdatedAt: chunkLabel.UpdatedAt,
		UserID:    p.user.ID,
		Name:      chunkLabel.Label,
	}

	if chunkLabel.IsDeleted {
		deleted, err := p.parent.repo.DeleteLabelForSync(p.ctx, entityLabel)
		if err != nil {
			return fmt.Errorf("failed to delete label %q: %w", chunkLabel.Label, err)
		}

		if deleted {
			p.updateOperations(singleUpdate)
		}
		return nil
	}

	isNew, labelNumID, err := p.parent.repo.UpsertLabelForSync(p.ctx, entityLabel)
	if err != nil {
		return fmt.Errorf("failed to upsert label %q: %w", chunkLabel.Label, err)
	}

	readerID, err := to.IntFromUnsigned(chunkLabel.TxLabelID)
	if err != nil {
		return fmt.Errorf("failed to convert label ID %d to int: %w", chunkLabel.TxLabelID, err)
	}

	p.updateOperations(to.IfThen(isNew, singleInsert).ElseThen(singleUpdate))
	err = p.updateSyncState(wdk.TxLabelEntityName, chunkLabel.UpdatedAt, 1, idDictionary{
		readerID: readerID,
		writerID: labelNumID,
	})
	if err != nil {
		return fmt.Errorf("failed to update sync state for label %q: %w", chunkLabel.Label, err)
	}

	return nil
}

func (p *chunkProcessor) upsertLabelMap(chunkLabelMap *wdk.TableTxLabelMap) error {
	transactionIDOnWriterSide, err := p.translateIDFromUnsigned(wdk.TransactionEntityName, chunkLabelMap.TransactionID)
	if err != nil {
		return fmt.Errorf("failed to translate transaction ID %d: %w", chunkLabelMap.TransactionID, err)
	}

	labelNumIDOrWriterSide, err := p.translateIDFromUnsigned(wdk.TxLabelEntityName, chunkLabelMap.TxLabelID)
	if err != nil {
		return fmt.Errorf("failed to translate label ID %d: %w", chunkLabelMap.TxLabelID, err)
	}

	labelEntity, err := p.getLabelByNumID(labelNumIDOrWriterSide)
	if err != nil {
		return fmt.Errorf("failed to get label by num ID %d: %w", labelNumIDOrWriterSide, err)
	}

	if labelEntity == nil {
		if chunkLabelMap.IsDeleted {
			// This is the case when the label has already been deleted on upsertLabel (along with matching label map).
			return nil
		} else {
			return fmt.Errorf("label with num ID %d not found for label map with transaction ID %d", labelNumIDOrWriterSide, chunkLabelMap.TransactionID)
		}
	}

	if labelEntity.UserID != p.user.ID {
		return fmt.Errorf("label with num ID %d belongs to user ID %d, but current user ID is %d", labelNumIDOrWriterSide, labelEntity.UserID, p.user.ID)
	}

	entityLabelMap := &entity.LabelMap{
		CreatedAt:     chunkLabelMap.CreatedAt,
		UpdatedAt:     chunkLabelMap.UpdatedAt,
		Name:          labelEntity.Name,
		UserID:        labelEntity.UserID,
		TransactionID: transactionIDOnWriterSide,
	}

	if chunkLabelMap.IsDeleted {
		deleted, err := p.parent.repo.DeleteLabelMapForSync(p.ctx, entityLabelMap)
		if err != nil {
			return fmt.Errorf("failed to delete label map for TxLabelID %d and TransactionID %d: %w", chunkLabelMap.TxLabelID, chunkLabelMap.TransactionID, err)
		}

		if deleted {
			p.updateOperations(singleUpdate)
		}
		return nil
	}

	isNew, err := p.parent.repo.UpsertLabelMapForSync(p.ctx, entityLabelMap)
	if err != nil {
		return fmt.Errorf("failed to upsert transaction label map for TxLabelID %d and TransactionID %d: %w", chunkLabelMap.TxLabelID, chunkLabelMap.TransactionID, err)
	}

	p.updateOperations(to.IfThen(isNew, singleInsert).ElseThen(singleUpdate))
	err = p.updateSyncState(wdk.TxLabelMapEntityName, chunkLabelMap.UpdatedAt, 1)
	if err != nil {
		return fmt.Errorf("failed to update sync state for label map with TxLabelID %d and TransactionID %d: %w", chunkLabelMap.TxLabelID, chunkLabelMap.TransactionID, err)
	}

	return nil
}

func (p *chunkProcessor) updateOperations(operations ...operation) {
	for _, op := range operations {
		p.result.Updates += op.updates
		p.result.Inserts += op.inserts
	}

}

type idDictionary struct {
	readerID int
	writerID uint
}

func (p *chunkProcessor) updateSyncState(entityName wdk.EntityName, updatedAt time.Time, count uint64, ids ...idDictionary) error {
	syncMapEntity, exists := p.syncState.SyncMap[entityName]
	if !exists {
		syncMapEntity = wdk.NewSyncMapEntity(entityName)
		p.syncState.SyncMap[entityName] = syncMapEntity
	}

	syncMapEntity.Count += count
	for _, id := range ids {
		writerIDInt, err := to.IntFromUnsigned(id.writerID)
		if err != nil {
			return fmt.Errorf("failed to convert writer ID %d to int: %w", id.writerID, err)
		}

		syncMapEntity.IDMap[id.readerID] = writerIDInt
	}

	if syncMapEntity.MaxUpdatedAt == nil || updatedAt.After(*syncMapEntity.MaxUpdatedAt) {
		syncMapEntity.MaxUpdatedAt = &updatedAt
	}

	return nil
}

func (p *chunkProcessor) translateID(entityName wdk.EntityName, readerID int) (uint, error) {
	syncMapEntity, exists := p.syncState.SyncMap[entityName]
	if !exists {
		return 0, fmt.Errorf("sync map entity %s not found", entityName)
	}

	writerID, ok := syncMapEntity.IDMap[readerID]
	if !ok {
		return 0, fmt.Errorf("no writer ID found for reader ID %d in entity %s", readerID, entityName)
	}

	writerIDUint, err := to.UInt(writerID)
	if err != nil {
		return 0, fmt.Errorf("failed to convert writer ID %d to uint: %w", writerID, err)
	}

	return writerIDUint, nil
}

func (p *chunkProcessor) translateIDFromUnsigned(entityName wdk.EntityName, readerID uint) (uint, error) {
	readerIDInt, err := to.IntFromUnsigned(readerID)
	if err != nil {
		return 0, fmt.Errorf("failed to convert reader ID %d to int: %w", readerID, err)
	}
	return p.translateID(entityName, readerIDInt)
}

// emptyChunk checks if the chunk is empty, meaning it has no row data to process.
// NOTE: The user pointer is not taken into account.
func (p *chunkProcessor) emptyChunk() bool {
	// TODO: Add more entities when implemented.
	return len(p.chunk.OutputBaskets) == 0 &&
		len(p.chunk.ProvenTxs) == 0 &&
		len(p.chunk.ProvenTxReqs) == 0 &&
		len(p.chunk.Transactions) == 0 &&
		len(p.chunk.Outputs) == 0 &&
		len(p.chunk.TxLabels) == 0 &&
		len(p.chunk.TxLabelMaps) == 0
}

func (p *chunkProcessor) getBasketNameByNumID(basketNumID uint) (string, error) {
	if name, ok := p.basketNameCache[basketNumID]; ok {
		return name, nil
	}

	basketName, err := p.parent.repo.FindBasketNameByNumIDForSync(p.ctx, basketNumID)
	if err != nil {
		return "", fmt.Errorf("failed to find output basket by num ID %d: %w", basketNumID, err)
	}

	p.basketNameCache[basketNumID] = basketName

	return basketName, nil
}

func (p *chunkProcessor) getLabelByNumID(labelNumID uint) (*entity.Label, error) {
	if label, ok := p.labelCache[labelNumID]; ok {
		return label, nil
	}

	label, err := p.parent.repo.FindLabelByNumIDForSync(p.ctx, labelNumID)
	if err != nil {
		return nil, fmt.Errorf("failed to find label by num ID %d: %w", labelNumID, err)
	}

	p.labelCache[labelNumID] = label

	return label, nil
}

// updateSyncStateOnDone updates the sync state when all the processing process is done.
// NOTE: By design, this method is called only once when a chunk is empty, meaning no more data to process.
// That's why it's crucial to call `processChunk` with an empty chunk at the end of the sync process.
// It resets the count (offsets) of all entities in the sync map and updates the `when` field to the maximum updated_at value.
// This way, the next sync will start from the latest state of the entities.
func (p *chunkProcessor) updateSyncStateOnDone() error {
	p.syncState.When = p.syncState.SyncMap.MaxUpdatedAt()
	for _, syncMapEntity := range p.syncState.SyncMap {
		syncMapEntity.Count = 0
	}

	if p.syncState.When != nil {
		// Ensure the `when` field is always "after" the last processed time
		// to avoid duplicate processing of a row (that has the maximum updated_at value) in the next sync.
		p.syncState.When = to.Ptr(p.syncState.When.Add(time.Nanosecond))
	}

	err := p.parent.repo.UpdateSyncState(p.ctx, p.syncState)
	if err != nil {
		return fmt.Errorf("failed to update sync state: %w", err)
	}

	return nil
}
