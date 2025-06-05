package sync

import "log/slog"

type Actions struct {
	*syncChunkAction
}

func New(logger *slog.Logger, repo Repository) *Actions {
	return &Actions{
		syncChunkAction: newSyncChunkAction(logger, repo),
	}
}
