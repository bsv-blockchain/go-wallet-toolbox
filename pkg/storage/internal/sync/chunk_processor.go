package sync

import (
	"context"
	"fmt"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"time"
)

type chunkProcessor struct {
	parent *processSyncChunk
	chunk  *wdk.SyncChunk
	result wdk.ProcessSyncChunkResult
	ctx    context.Context
}

func newChunkProcessor(ctx context.Context, parent *processSyncChunk, chunk *wdk.SyncChunk) *chunkProcessor {
	return &chunkProcessor{
		ctx:    ctx,
		parent: parent,
		chunk:  chunk,
	}
}

func (p *chunkProcessor) process() error {
	if p.chunk.User != nil {
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

	if p.chunk.User.UserID != chunkUserInCurrentDB.UserID {
		// TODO: Double check that. How this could even work in production where it's very likely that primary keys of different DBs with different users DO NOT match.
		return fmt.Errorf("user ID mismatch: chunk user ID %d does not match found user ID %d", p.chunk.User.UserID, chunkUserInCurrentDB.UserID)
	}

	currentDBHasNewerVersion := chunkUserInCurrentDB.UpdatedAt.After(p.chunk.User.UpdatedAt)
	needsActiveStorageUpdate := chunkUserInCurrentDB.ActiveStorage == "" && p.chunk.User.ActiveStorage != ""

	if currentDBHasNewerVersion && !needsActiveStorageUpdate {
		return nil // No update needed, the current DB user is newer and already has an active storage.
	}

	var newerUpdatedAt time.Time
	if currentDBHasNewerVersion {
		newerUpdatedAt = chunkUserInCurrentDB.UpdatedAt
	} else {
		newerUpdatedAt = p.chunk.User.UpdatedAt
	}

	err = p.parent.repo.UpdateUser(p.ctx, p.chunk.User.UserID, p.chunk.User.ActiveStorage, newerUpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	p.updateResult(newerUpdatedAt, 1, 0)
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
