package sync

import (
	"log/slog"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type Actions struct {
	*syncChunkAction
	*findOrInsertSyncState
	*processSyncChunk
}

func New(logger *slog.Logger, repo Repository, random wdk.Randomizer) *Actions {
	return &Actions{
		syncChunkAction:       newSyncChunkAction(logger, repo),
		findOrInsertSyncState: newFindOrInsertSyncState(logger, repo, random),
		processSyncChunk:      newProcessSyncChunk(logger, repo),
	}
}
