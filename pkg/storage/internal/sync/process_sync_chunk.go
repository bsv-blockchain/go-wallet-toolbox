package sync

import (
	"context"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"log/slog"
)

type processSyncChunk struct {
	repo   Repository
	logger *slog.Logger
}

func newProcessSyncChunk(logger *slog.Logger, repo Repository) *processSyncChunk {
	return &processSyncChunk{
		logger: logger,
		repo:   repo,
	}
}

func (p *processSyncChunk) Process(ctx context.Context, args wdk.RequestSyncChunkArgs, chunk *wdk.SyncChunk) (*wdk.ProcessSyncChunkResult, error) {
	panic("Not implemented yet")
}
