package sync

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
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
	user, err := p.repo.FindUser(ctx, args.IdentityKey)
	if err != nil {
		return nil, fmt.Errorf("failed to find user: %w", err)
	}

	if user == nil {
		return nil, fmt.Errorf("user with identity key %s not found", args.IdentityKey)
	}

	processor := newChunkProcessor(ctx, p, chunk)
	err = processor.process()
	if err != nil {
		return nil, fmt.Errorf("failed to process chunk: %w", err)
	}

	return &processor.result, nil
}
