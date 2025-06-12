package sync

import (
	"context"
	"fmt"
	"time"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
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
		if p.chunk.User.UserID != p.user.ID {
			return fmt.Errorf("chunk user ID %d does not match current user ID %d", p.chunk.User.UserID, p.user.ID)
		}

		if err = p.mergeUser(); err != nil {
			return fmt.Errorf("failed to merge user: %w", err)
		}
	}

	if p.emptyChunk() {
		p.result.Done = true
		return nil // No data to process, return early.
	}

	for _, basket := range p.chunk.OutputBaskets {
		if err = p.mergeBaskets(basket); err != nil {
			return fmt.Errorf("failed to merge basket %q: %w", basket.Name, err)
		}
	}

	err = p.parent.repo.UpdateSyncState(p.ctx, p.syncState)
	if err != nil {
		return fmt.Errorf("failed to update sync state: %w", err)
	}

	return nil
}

func (p *chunkProcessor) mergeUser() error {
	chunkUserInCurrentDB, err := p.parent.repo.FindUser(p.ctx, p.chunk.User.IdentityKey)
	if err != nil {
		return fmt.Errorf("failed to find chunk user: %w", err)
	}

	if chunkUserInCurrentDB == nil {
		return fmt.Errorf("chunk user not found for userID %d", p.chunk.User.UserID)
	}

	if chunkUserInCurrentDB.ID != p.chunk.User.UserID {
		return fmt.Errorf("chunk user ID %d does not match current DB user ID %d", p.chunk.User.UserID, chunkUserInCurrentDB.ID)
	}

	currentDBHasNewerVersion := chunkUserInCurrentDB.UpdatedAt.After(p.chunk.User.UpdatedAt)
	if currentDBHasNewerVersion {
		return nil // No update needed, the current DB user is newer and already has an active storage.
	}

	err = p.parent.repo.UpdateUser(p.ctx, p.chunk.User.UserID, p.chunk.User.ActiveStorage, p.chunk.User.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	p.updateResult(p.chunk.User.UpdatedAt, 1, 0)
	return nil
}

func (p *chunkProcessor) updateResult(updatedAt time.Time, updates, inserts int) {
	p.result.Updates += updates
	p.result.Inserts += inserts

	if p.result.MaxUpdatedAt == nil || updatedAt.After(*p.result.MaxUpdatedAt) {
		p.result.MaxUpdatedAt = &updatedAt
	}
}

func (p *chunkProcessor) mergeBaskets(chunkBasket *wdk.TableOutputBasket) error {
	upserted, err := p.parent.repo.UpsertOutputBasket(p.ctx, p.user.ID, chunkBasket.BasketConfiguration)
	if err != nil {
		return fmt.Errorf("failed to upsert output basket %q: %w", chunkBasket.Name, err)
	}

	isNew := upserted.CreatedAt == upserted.UpdatedAt
	if isNew {
		p.updateResult(upserted.UpdatedAt, 0, 1)
	} else {
		p.updateResult(upserted.UpdatedAt, 1, 0)
	}

	syncMapEntity := p.syncState.SyncMap[wdk.OutputBasketEntityName]
	syncMapEntity.Count += 1

	// TODO: Most probably, we need to update the sync state (with IDMap) - But I postpone this until it is actually needed.

	return nil
}

// emptyChunk checks if the chunk is empty, meaning it has no row data to process.
// NOTE: The user pointer is not taken into account.
func (p *chunkProcessor) emptyChunk() bool {
	// TODO: Add more entities when implemented.
	return len(p.chunk.OutputBaskets) == 0
}
