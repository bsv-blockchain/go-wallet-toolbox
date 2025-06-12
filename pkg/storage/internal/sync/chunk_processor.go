package sync

import (
	"context"
	"fmt"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"time"
)

type chunkProcessor struct {
	parent    *processSyncChunk
	chunk     *wdk.SyncChunk
	result    wdk.ProcessSyncChunkResult
	ctx       context.Context
	syncState *wdk.TableSyncState
}

func newChunkProcessor(ctx context.Context, parent *processSyncChunk, chunk *wdk.SyncChunk) *chunkProcessor {
	return &chunkProcessor{
		ctx:    ctx,
		parent: parent,
		chunk:  chunk,
	}
}

func (p *chunkProcessor) process() error {
	// TODO: Get SyncState from DB

	if p.chunk.User != nil {
		// TODO Check if the syncState actually refers to the user in the chunk.

		if err := p.mergeUser(); err != nil {
			return fmt.Errorf("failed to merge user: %w", err)
		}
	}

	//for _, basket := range p.chunk.OutputBaskets {
	//	if err := p.mergeBaskets(basket); err != nil {
	//		return fmt.Errorf("failed to merge basket %q: %w", basket.Name, err)
	//	}
	//}

	return nil
}

func (p *chunkProcessor) mergeUser() error {
	chunkUserInCurrentDB, err := p.parent.repo.FindUser(p.ctx, p.chunk.User.IdentityKey)
	if err != nil {
		return fmt.Errorf("failed to find chunk user: %w", err)
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
	panic("implement me")
}
