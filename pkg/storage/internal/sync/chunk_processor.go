package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	pkgentity "github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/txutils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/optional"
	"github.com/go-softwarelab/common/pkg/slices"
	"github.com/go-softwarelab/common/pkg/slogx"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/go-softwarelab/common/pkg/types"
)

type ChunkProcessor struct {
	repo            Repository
	chunk           *wdk.SyncChunk
	result          wdk.ProcessSyncChunkResult
	ctx             context.Context
	user            *pkgentity.User
	args            *wdk.RequestSyncChunkArgs
	syncState       *entity.SyncState
	basketNameCache map[uint]string
	labelCache      map[uint]*entity.Label
	tagCache        map[uint]*entity.Tag
	logger          *slog.Logger
}

func NewChunkProcessor(ctx context.Context, logger *slog.Logger, repo Repository, chunk *wdk.SyncChunk, args *wdk.RequestSyncChunkArgs, user *pkgentity.User) *ChunkProcessor {
	logger = logging.Child(logger, "chunkProcessor").With(
		slog.String("fromStorageIdentityKey", args.FromStorageIdentityKey),
		slog.String("toStorageIdentityKey", args.ToStorageIdentityKey),
		slog.Int("userID", user.ID),
	)

	return &ChunkProcessor{
		ctx:             ctx,
		repo:            repo,
		chunk:           chunk,
		args:            args,
		user:            user,
		basketNameCache: map[uint]string{},
		labelCache:      map[uint]*entity.Label{},
		tagCache:        map[uint]*entity.Tag{},
		logger:          logger,
	}
}

func (p *ChunkProcessor) Process() (*wdk.ProcessSyncChunkResult, error) {
	p.logger.InfoContext(p.ctx, "processing sync chunk")
	syncState, err := p.repo.FindSyncState(p.ctx, p.user.ID, p.args.FromStorageIdentityKey)
	if err != nil {
		return nil, fmt.Errorf("failed to find sync state: %w", err)
	}

	if syncState == nil {
		return nil, fmt.Errorf("sync state not found for userID %d and storage %s", p.user.ID, p.args.FromStorageIdentityKey)
	}

	p.syncState = syncState

	if p.chunk.User != nil {
		p.logger.InfoContext(p.ctx, "merging user from chunk")
		if err = p.mergeUser(); err != nil {
			return nil, fmt.Errorf("failed to merge user: %w", err)
		}
	}

	if p.emptyChunk() {
		p.logger.InfoContext(p.ctx, "empty chunk, which means sync is done, updating sync state")
		err = p.updateSyncStateOnDone()
		if err != nil {
			return nil, fmt.Errorf("failed to update sync state on done: %w", err)
		}

		p.result.MaxUpdatedAt = p.syncState.When
		p.result.Done = true

		return &p.result, nil
	}

	for _, basket := range p.chunk.OutputBaskets {
		if err = p.upsertBasket(basket); err != nil {
			return nil, err
		}
	}

	for _, provenTxReq := range p.chunk.ProvenTxReqs {
		if err = p.upsertProvenTxReqs(provenTxReq); err != nil {
			return nil, err
		}
	}

	for _, provenTx := range p.chunk.ProvenTxs {
		if err = p.upsertProvenTx(provenTx); err != nil {
			return nil, err
		}
	}

	for _, transaction := range p.chunk.Transactions {
		if err = p.upsertTransaction(transaction); err != nil {
			return nil, err
		}
	}

	for _, output := range p.chunk.Outputs {
		if err = p.upsertOutput(output); err != nil {
			return nil, fmt.Errorf("failed to upsert output: %w", err)
		}
	}

	for _, label := range p.chunk.TxLabels {
		if err = p.upsertLabel(label); err != nil {
			return nil, fmt.Errorf("failed to upsert label: %w", err)
		}
	}

	for _, labelMap := range p.chunk.TxLabelMaps {
		if err = p.upsertLabelMap(labelMap); err != nil {
			return nil, fmt.Errorf("failed to upsert label map: %w", err)
		}
	}

	for _, tag := range p.chunk.OutputTags {
		if err = p.upsertTag(tag); err != nil {
			return nil, fmt.Errorf("failed to upsert tag: %w", err)
		}
	}

	for _, tagMap := range p.chunk.OutputTagMaps {
		if err = p.upsertTagMap(tagMap); err != nil {
			return nil, fmt.Errorf("failed to upsert tag map: %w", err)
		}
	}

	p.logger.DebugContext(p.ctx, "updating sync state on chunk processed")
	err = p.repo.UpdateSyncState(p.ctx, p.syncState)
	if err != nil {
		return nil, fmt.Errorf("failed to update sync state: %w", err)
	}

	p.result.MaxUpdatedAt = p.syncState.SyncMap.MaxUpdatedAt()

	return &p.result, nil
}

func (p *ChunkProcessor) mergeUser() error {
	if p.chunk.User.IdentityKey != p.user.IdentityKey {
		return fmt.Errorf("chunk user identity key %s does not match current user identity key %s", p.chunk.User.IdentityKey, p.user.IdentityKey)
	}

	currentDBHasOlderVersion := p.user.UpdatedAt.Before(p.chunk.User.UpdatedAt)
	if !currentDBHasOlderVersion {
		return nil // No update needed
	}

	err := p.repo.UpdateUserForSync(p.ctx, p.user.ID, p.chunk.User.ActiveStorage, p.chunk.User.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to update user %d: %w", p.chunk.User.UserID, err)
	}

	p.incrementOperations(false)
	return nil
}

func (p *ChunkProcessor) upsertBasket(chunkBasket *wdk.TableOutputBasket) error {
	if p.chunk.User != nil && p.chunk.User.UserID != chunkBasket.UserID {
		return fmt.Errorf("chunk basket user ID %d does not match chunk user ID %d", chunkBasket.UserID, p.chunk.User.UserID)
	}

	p.logger.DebugContext(p.ctx, "upserting basket", slogx.String("name", chunkBasket.Name))

	isNew, basketNumID, err := p.repo.UpsertOutputBasketForSync(p.ctx, pkgentity.OutputBasket{
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
	p.incrementOperations(isNew)
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

func (p *ChunkProcessor) upsertProvenTxReqs(chunkProvenTxReq *wdk.TableProvenTxReq) error {
	p.logger.DebugContext(p.ctx, "upserting proven tx req", slog.String("txid", chunkProvenTxReq.TxID))

	historyNotes, err := p.getHistoryNotes(chunkProvenTxReq.TxID, chunkProvenTxReq.History)
	if err != nil {
		return fmt.Errorf("failed to get history notes for TxID %q: %w", chunkProvenTxReq.TxID, err)
	}

	isNew, err := p.repo.UpsertKnownTxForSync(p.ctx, &pkgentity.KnownTx{
		CreatedAt: chunkProvenTxReq.CreatedAt,
		UpdatedAt: chunkProvenTxReq.UpdatedAt,
		TxID:      chunkProvenTxReq.TxID,
		Status:    chunkProvenTxReq.Status,
		Attempts:  chunkProvenTxReq.Attempts,
		Notified:  chunkProvenTxReq.Notified,
		RawTx:     chunkProvenTxReq.RawTx,
		InputBEEF: chunkProvenTxReq.InputBEEF,
		TxNotes:   historyNotes,
	})
	if err != nil {
		return fmt.Errorf("failed to upsert proven tx req for TxID %q: %w", chunkProvenTxReq.TxID, err)
	}

	p.incrementOperations(isNew)
	err = p.updateSyncState(wdk.ProvenTxReqEntityName, chunkProvenTxReq.UpdatedAt, 1)
	if err != nil {
		return fmt.Errorf("failed to update sync state for proven tx req %q: %w", chunkProvenTxReq.TxID, err)
	}

	return nil
}

func (p *ChunkProcessor) getHistoryNotes(txID string, encoded string) ([]*pkgentity.TxHistoryNote, error) {
	const minLength = 12 // len of `{"notes":[]}`
	if len(encoded) < minLength {
		return nil, nil
	}

	var notesObj struct {
		Notes []wdk.HistoryNote `json:"notes"`
	}
	err := json.Unmarshal([]byte(encoded), &notesObj)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal history notes: %w", err)
	}

	return slices.Map(notesObj.Notes, func(note wdk.HistoryNote) *pkgentity.TxHistoryNote {
		// TODO: UserIDs can mismatch because the translation is not implemented (in TS also)

		return &pkgentity.TxHistoryNote{
			HistoryNote: note,
			TxID:        txID,
		}
	}), nil
}

func (p *ChunkProcessor) upsertProvenTx(chunkProvenTx *wdk.TableProvenTx) error {
	p.logger.DebugContext(p.ctx, "upserting proven tx", slog.String("txid", chunkProvenTx.TxID))

	isNew, err := p.repo.UpsertKnownTxForSync(p.ctx, &pkgentity.KnownTx{
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

	p.incrementOperations(isNew)
	err = p.updateSyncState(wdk.ProvenTxEntityName, chunkProvenTx.UpdatedAt, 1)
	if err != nil {
		return fmt.Errorf("failed to update sync state for proven tx %q: %w", chunkProvenTx.TxID, err)
	}

	return nil
}

func (p *ChunkProcessor) upsertTransaction(chunkTransaction *wdk.TableTransaction) error {
	if p.chunk.User != nil && p.chunk.User.UserID != chunkTransaction.UserID {
		return fmt.Errorf("chunk transaction user ID %d does not match chunk user ID %d", chunkTransaction.UserID, p.chunk.User.UserID)
	}

	p.logger.DebugContext(p.ctx, "upserting transaction", slog.String("reference", string(chunkTransaction.Reference)))

	isNew, transactionID, err := p.repo.UpsertTransactionForSync(p.ctx, &pkgentity.Transaction{
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

	p.incrementOperations(isNew)
	err = p.updateSyncState(wdk.TransactionEntityName, chunkTransaction.UpdatedAt, 1, idDictionary{
		readerID: readerID,
		writerID: transactionID,
	})
	if err != nil {
		return fmt.Errorf("failed to update sync state for transaction with reference %q: %w", chunkTransaction.Reference, err)
	}

	return nil
}

func (p *ChunkProcessor) upsertOutput(chunkOutput *wdk.TableOutput) error {
	if p.chunk.User != nil && p.chunk.User.UserID != chunkOutput.UserID {
		return fmt.Errorf("chunk output user ID %d does not match chunk user ID %d", chunkOutput.UserID, p.chunk.User.UserID)
	}

	p.logger.DebugContext(p.ctx, "upserting output", logging.Number("txid", chunkOutput.TransactionID), logging.Number("vout", chunkOutput.Vout))

	var basketName *string
	if chunkOutput.BasketID != nil {
		basketIDOnWriterSide, err := translateID(p, wdk.OutputBasketEntityName, *chunkOutput.BasketID)
		if err != nil {
			return fmt.Errorf("failed to translate basket ID %d: %w", *chunkOutput.BasketID, err)
		}

		name, err := p.getBasketNameByNumID(basketIDOnWriterSide)
		if err != nil {
			return fmt.Errorf("failed to get basket name for basket ID %d: %w", basketIDOnWriterSide, err)
		}

		basketName = &name
	}

	transactionIDOnWriterSide, err := translateID(p, wdk.TransactionEntityName, chunkOutput.TransactionID)
	if err != nil {
		return fmt.Errorf("failed to translate transaction ID %d: %w", chunkOutput.TransactionID, err)
	}

	var spentByTransactionIDOnWriterSide *uint
	if chunkOutput.SpentBy != nil {
		spentByTransactionID, err := translateID(p, wdk.TransactionEntityName, *chunkOutput.SpentBy)
		if err != nil {
			return fmt.Errorf("failed to translate spent by transaction ID %d: %w", *chunkOutput.SpentBy, err)
		}
		spentByTransactionIDOnWriterSide = &spentByTransactionID
	}

	output := &pkgentity.Output{
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

		output.UserUTXO = &pkgentity.UserUTXO{
			UserID:             p.user.ID,
			BasketName:         wdk.BasketNameForChange,
			Satoshis:           satoshis,
			EstimatedInputSize: txutils.EstimatedInputSizeByType(wdk.OutputType(output.Type)),
			CreatedAt:          chunkOutput.CreatedAt,
			ReservedByID:       nil, //TODO: Talk to Damian how to deal with this - as it cannot be deduced from the output.
		}
	}

	isNew, outputID, err := p.repo.UpsertOutputForSync(p.ctx, output)
	if err != nil {
		return fmt.Errorf("failed to upsert output for transaction ID %d, vout %d: %w", chunkOutput.TransactionID, chunkOutput.Vout, err)
	}

	readerID, err := to.IntFromUnsigned(chunkOutput.OutputID)
	if err != nil {
		return fmt.Errorf("failed to convert output ID %d to int: %w", chunkOutput.OutputID, err)
	}

	p.incrementOperations(isNew)
	err = p.updateSyncState(wdk.OutputEntityName, chunkOutput.UpdatedAt, 1, idDictionary{
		readerID: readerID,
		writerID: outputID,
	})
	if err != nil {
		return fmt.Errorf("failed to update sync state for output with transaction ID %d and vout %d: %w", chunkOutput.TransactionID, chunkOutput.Vout, err)
	}

	return nil
}

func (p *ChunkProcessor) upsertLabel(chunkLabel *wdk.TableTxLabel) error {
	if p.chunk.User != nil && p.chunk.User.UserID != chunkLabel.UserID {
		return fmt.Errorf("chunk label user ID %d does not match chunk user ID %d", chunkLabel.UserID, p.chunk.User.UserID)
	}

	p.logger.DebugContext(p.ctx, "upserting label", slog.String("name", chunkLabel.Label))

	entityLabel := &entity.Label{
		CreatedAt: chunkLabel.CreatedAt,
		UpdatedAt: chunkLabel.UpdatedAt,
		UserID:    p.user.ID,
		Name:      chunkLabel.Label,
	}

	if chunkLabel.IsDeleted {
		deleted, err := p.repo.DeleteLabelForSync(p.ctx, entityLabel)
		if err != nil {
			return fmt.Errorf("failed to delete label %q: %w", chunkLabel.Label, err)
		}

		if deleted {
			p.incrementOperations(false)
		}
		return nil
	}

	isNew, labelNumID, err := p.repo.UpsertLabelForSync(p.ctx, entityLabel)
	if err != nil {
		return fmt.Errorf("failed to upsert label %q: %w", chunkLabel.Label, err)
	}

	readerID, err := to.IntFromUnsigned(chunkLabel.TxLabelID)
	if err != nil {
		return fmt.Errorf("failed to convert label ID %d to int: %w", chunkLabel.TxLabelID, err)
	}

	p.incrementOperations(isNew)
	err = p.updateSyncState(wdk.TxLabelEntityName, chunkLabel.UpdatedAt, 1, idDictionary{
		readerID: readerID,
		writerID: labelNumID,
	})
	if err != nil {
		return fmt.Errorf("failed to update sync state for label %q: %w", chunkLabel.Label, err)
	}

	return nil
}

func (p *ChunkProcessor) upsertLabelMap(chunkLabelMap *wdk.TableTxLabelMap) error {
	p.logger.DebugContext(p.ctx, "upserting label map", logging.Number("txLabelID", chunkLabelMap.TxLabelID), logging.Number("transactionID", chunkLabelMap.TransactionID))

	transactionIDOnWriterSide, err := translateID(p, wdk.TransactionEntityName, chunkLabelMap.TransactionID)
	if err != nil {
		return fmt.Errorf("failed to translate transaction ID %d: %w", chunkLabelMap.TransactionID, err)
	}

	labelNumIDOrWriterSide, err := translateID(p, wdk.TxLabelEntityName, chunkLabelMap.TxLabelID)
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
		deleted, err := p.repo.DeleteLabelMapForSync(p.ctx, entityLabelMap)
		if err != nil {
			return fmt.Errorf("failed to delete label map for TxLabelID %d and TransactionID %d: %w", chunkLabelMap.TxLabelID, chunkLabelMap.TransactionID, err)
		}

		if deleted {
			p.incrementOperations(false)
		}
		return nil
	}

	isNew, err := p.repo.UpsertLabelMapForSync(p.ctx, entityLabelMap)
	if err != nil {
		return fmt.Errorf("failed to upsert transaction label map for TxLabelID %d and TransactionID %d: %w", chunkLabelMap.TxLabelID, chunkLabelMap.TransactionID, err)
	}

	p.incrementOperations(isNew)
	err = p.updateSyncState(wdk.TxLabelMapEntityName, chunkLabelMap.UpdatedAt, 1)
	if err != nil {
		return fmt.Errorf("failed to update sync state for label map with TxLabelID %d and TransactionID %d: %w", chunkLabelMap.TxLabelID, chunkLabelMap.TransactionID, err)
	}

	return nil
}

func (p *ChunkProcessor) upsertTag(chunkTag *wdk.TableOutputTag) error {
	if p.chunk.User != nil && p.chunk.User.UserID != chunkTag.UserID {
		return fmt.Errorf("chunk tag user ID %d does not match chunk user ID %d", chunkTag.UserID, p.chunk.User.UserID)
	}

	p.logger.DebugContext(p.ctx, "upserting tag", slog.String("name", chunkTag.Tag))

	entityTag := &entity.Tag{
		CreatedAt: chunkTag.CreatedAt,
		UpdatedAt: chunkTag.UpdatedAt,
		UserID:    p.user.ID,
		Name:      chunkTag.Tag,
	}

	if chunkTag.IsDeleted {
		deleted, err := p.repo.DeleteTagForSync(p.ctx, entityTag)
		if err != nil {
			return fmt.Errorf("failed to delete tag %q: %w", chunkTag.Tag, err)
		}

		if deleted {
			p.incrementOperations(false)
		}
		return nil
	}

	isNew, tagNumID, err := p.repo.UpsertTagForSync(p.ctx, entityTag)
	if err != nil {
		return fmt.Errorf("failed to upsert tag %q: %w", chunkTag.Tag, err)
	}

	readerID, err := to.IntFromUnsigned(chunkTag.OutputTagID)
	if err != nil {
		return fmt.Errorf("failed to convert tag ID %d to int: %w", chunkTag.OutputTagID, err)
	}

	p.incrementOperations(isNew)
	err = p.updateSyncState(wdk.OutputTagEntityName, chunkTag.UpdatedAt, 1, idDictionary{
		readerID: readerID,
		writerID: tagNumID,
	})
	if err != nil {
		return fmt.Errorf("failed to update sync state for tag %q: %w", chunkTag.Tag, err)
	}

	return nil
}

func (p *ChunkProcessor) upsertTagMap(chunkTagMap *wdk.TableOutputTagMap) error {
	p.logger.DebugContext(p.ctx, "upserting tag map", logging.Number("outputTagID", chunkTagMap.OutputTagID), logging.Number("outputID", chunkTagMap.OutputID))

	outputIDOnWriterSide, err := translateID(p, wdk.OutputEntityName, chunkTagMap.OutputID)
	if err != nil {
		return fmt.Errorf("failed to translate output ID %d: %w", chunkTagMap.OutputID, err)
	}

	tagNumIDOrWriterSide, err := translateID(p, wdk.OutputTagEntityName, chunkTagMap.OutputTagID)
	if err != nil {
		return fmt.Errorf("failed to translate tag ID %d: %w", chunkTagMap.OutputTagID, err)
	}

	tagEntity, err := p.getTagByNumID(tagNumIDOrWriterSide)
	if err != nil {
		return fmt.Errorf("failed to get tag by num ID %d: %w", tagNumIDOrWriterSide, err)
	}

	if tagEntity == nil {
		if chunkTagMap.IsDeleted {
			// This is the case when the tag has already been deleted on upsertTag (along with matching tag map).
			return nil
		} else {
			return fmt.Errorf("tag with num ID %d not found for tag map with output ID %d", tagNumIDOrWriterSide, chunkTagMap.OutputID)
		}
	}

	if tagEntity.UserID != p.user.ID {
		return fmt.Errorf("tag with num ID %d belongs to user ID %d, but current user ID is %d", tagNumIDOrWriterSide, tagEntity.UserID, p.user.ID)
	}

	entityTagMap := &entity.TagMap{
		CreatedAt: chunkTagMap.CreatedAt,
		UpdatedAt: chunkTagMap.UpdatedAt,
		Name:      tagEntity.Name,
		UserID:    tagEntity.UserID,
		OutputID:  outputIDOnWriterSide,
	}

	if chunkTagMap.IsDeleted {
		deleted, err := p.repo.DeleteTagMapForSync(p.ctx, entityTagMap)
		if err != nil {
			return fmt.Errorf("failed to delete tag map for OutputTagID %d and OutputID %d: %w", chunkTagMap.OutputTagID, chunkTagMap.OutputID, err)
		}

		if deleted {
			p.incrementOperations(false)
		}
		return nil
	}

	isNew, err := p.repo.UpsertTagMapForSync(p.ctx, entityTagMap)
	if err != nil {
		return fmt.Errorf("failed to upsert output tag map for OutputTagID %d and OutputID %d: %w", chunkTagMap.OutputTagID, chunkTagMap.OutputID, err)
	}

	p.incrementOperations(isNew)
	err = p.updateSyncState(wdk.OutputTagMapEntityName, chunkTagMap.UpdatedAt, 1)
	if err != nil {
		return fmt.Errorf("failed to update sync state for tag map for OutputTagID %d and OutputID %d: %w", chunkTagMap.OutputTagID, chunkTagMap.OutputID, err)
	}

	return nil
}

func (p *ChunkProcessor) incrementOperations(isCreateOperation bool) {
	if isCreateOperation {
		p.result.Inserts++
	} else {
		p.result.Updates++
	}
}

type idDictionary struct {
	readerID int
	writerID uint
}

func (p *ChunkProcessor) updateSyncState(entityName wdk.EntityName, updatedAt time.Time, count uint64, ids ...idDictionary) error {
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

// emptyChunk checks if the chunk is empty, meaning it has no row data to process.
// NOTE: The user pointer is not taken into account.
func (p *ChunkProcessor) emptyChunk() bool {
	return len(p.chunk.OutputBaskets) == 0 &&
		len(p.chunk.ProvenTxs) == 0 &&
		len(p.chunk.ProvenTxReqs) == 0 &&
		len(p.chunk.Transactions) == 0 &&
		len(p.chunk.Outputs) == 0 &&
		len(p.chunk.TxLabels) == 0 &&
		len(p.chunk.TxLabelMaps) == 0 &&
		len(p.chunk.OutputTags) == 0 &&
		len(p.chunk.OutputTagMaps) == 0
}

func (p *ChunkProcessor) getBasketNameByNumID(basketNumID uint) (string, error) {
	if name, ok := p.basketNameCache[basketNumID]; ok {
		return name, nil
	}

	basketName, err := p.repo.FindBasketNameByNumIDForSync(p.ctx, basketNumID)
	if err != nil {
		return "", fmt.Errorf("failed to find output basket by num ID %d: %w", basketNumID, err)
	}

	p.basketNameCache[basketNumID] = basketName

	return basketName, nil
}

func (p *ChunkProcessor) getLabelByNumID(labelNumID uint) (*entity.Label, error) {
	if label, ok := p.labelCache[labelNumID]; ok {
		return label, nil
	}

	label, err := p.repo.FindLabelByNumIDForSync(p.ctx, labelNumID)
	if err != nil {
		return nil, fmt.Errorf("failed to find label by num ID %d: %w", labelNumID, err)
	}

	p.labelCache[labelNumID] = label

	return label, nil
}

func (p *ChunkProcessor) getTagByNumID(tagNumID uint) (*entity.Tag, error) {
	if tag, ok := p.tagCache[tagNumID]; ok {
		return tag, nil
	}

	tag, err := p.repo.FindTagByNumIDForSync(p.ctx, tagNumID)
	if err != nil {
		return nil, fmt.Errorf("failed to find tag by num ID %d: %w", tagNumID, err)
	}

	p.tagCache[tagNumID] = tag

	return tag, nil
}

// updateSyncStateOnDone updates the sync state when all the processing process is done.
// NOTE: By design, this method is called only once when a chunk is empty, meaning no more data to process.
// That's why it's crucial to call `processChunk` with an empty chunk at the end of the sync process.
// It resets the count (offsets) of all entities in the sync map and updates the `when` field to the maximum updated_at value.
// This way, the next sync will start from the latest state of the entities.
func (p *ChunkProcessor) updateSyncStateOnDone() error {
	p.syncState.When = p.syncState.SyncMap.MaxUpdatedAt()
	for _, syncMapEntity := range p.syncState.SyncMap {
		syncMapEntity.Count = 0
	}

	if p.syncState.When != nil {
		// Ensure the `when` field is always "after" the last processed time
		// to avoid duplicate processing of a row (that has the maximum updated_at value) in the next sync.
		p.syncState.When = to.Ptr(p.syncState.When.Add(time.Nanosecond))
	}

	err := p.repo.UpdateSyncState(p.ctx, p.syncState)
	if err != nil {
		return fmt.Errorf("failed to update sync state: %w", err)
	}

	return nil
}

func translateID[T types.Number](p *ChunkProcessor, entityName wdk.EntityName, readerID T) (uint, error) {
	syncMapEntity, exists := p.syncState.SyncMap[entityName]
	if !exists {
		return 0, fmt.Errorf("sync map entity %s not found", entityName)
	}

	readerIDInt, err := to.Int(readerID)
	if err != nil {
		return 0, fmt.Errorf("failed to convert reader ID %v to int: %w", readerID, err)
	}

	writerID, ok := syncMapEntity.IDMap[readerIDInt]
	if !ok {
		return 0, fmt.Errorf("no writer ID found for reader ID %v in entity %s", readerID, entityName)
	}

	writerIDUint, err := to.UInt(writerID)
	if err != nil {
		return 0, fmt.Errorf("failed to convert writer ID %d to uint: %w", writerID, err)
	}

	return writerIDUint, nil
}
