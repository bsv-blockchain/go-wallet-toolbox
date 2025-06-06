package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/logging"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/must"
	"github.com/go-softwarelab/common/pkg/to"
)

const (
	maxIterations = 1000
	minPageSize   = 10
)

type syncChunkAction struct {
	logger   *slog.Logger
	repo     Repository
	chunkers []Chunker
}

func newSyncChunkAction(logger *slog.Logger, repo Repository) *syncChunkAction {
	return &syncChunkAction{
		logger:   logging.Child(logger, "getSyncChunk"),
		repo:     repo,
		chunkers: all(repo),
	}
}

func (s *syncChunkAction) GetSyncChunk(ctx context.Context, args *wdk.RequestSyncChunkArgs) (*wdk.SyncChunk, error) {
	user, err := s.repo.FindUser(ctx, args.IdentityKey)
	if err != nil {
		return nil, fmt.Errorf("cannot find user: %w", err)
	}

	if user == nil {
		return nil, fmt.Errorf("user with identity key %s not found", args.IdentityKey)
	}

	chunk := &wdk.SyncChunk{
		FromStorageIdentityKey: args.FromStorageIdentityKey,
		ToStorageIdentityKey:   args.ToStorageIdentityKey,
		UserIdentityKey:        args.IdentityKey,
	}

	if args.Since == nil || user.UpdatedAt.After(*args.Since) {
		chunk.User = user
	}

	if err = s.process(ctx, args, user.UserID, chunk); err != nil {
		return nil, fmt.Errorf("failed to process sync chunk: %w", err)
	}

	return chunk, nil
}

func (s *syncChunkAction) process(ctx context.Context, args *wdk.RequestSyncChunkArgs, userID int, result *wdk.SyncChunk) error {
	var err error
	var itemsCounter uint64

	offsetsLookup := s.makeOffsetsLookup(args)

	for _, chunker := range s.chunkers {
		if !chunker.IsApplicable(offsetsLookup) {
			continue
		}

		var page = chunker.FirstPage(offsetsLookup)

		for range maxIterations {
			if ctx.Err() != nil {
				return fmt.Errorf("context canceled, aborting: %w", err)
			}

			freeSlots := args.MaxItems - itemsCounter
			limit := to.ValueBetween(args.MaxItems/chunker.MaxDivider(), minPageSize, freeSlots)
			page.Limit = must.ConvertToIntFromUnsigned(limit)

			num, err := chunker.Process(ctx, userID, page, args.Since, result)
			if err != nil {
				return fmt.Errorf("chunker %s failed: %w", chunker.Name(), err)
			}

			if num == 0 {
				break
			}

			itemsCounter += num
			reachedMax := itemsCounter >= args.MaxItems || s.approxJSONSize(result) >= args.MaxRoughSize
			if reachedMax {
				// NOTE: This breaks also the "chunkers" loop because we don't have any more free slots.
				return nil
			}

			page.Next()
		}
	}

	return nil
}

func (s *syncChunkAction) makeOffsetsLookup(args *wdk.RequestSyncChunkArgs) OffsetsLookup {
	offsetsLookup := make(OffsetsLookup)
	for _, it := range args.Offsets {
		offsetsLookup[it.Name] = it.Offset
	}
	return offsetsLookup
}

func (s *syncChunkAction) approxJSONSize(chunk *wdk.SyncChunk) uint64 {
	// TODO it could be less precise and use a more efficient way to estimate size
	b, err := json.Marshal(chunk)
	if err != nil {
		s.logger.Warn("failed to marshal sync chunk for size estimation", slog.String("error", err.Error()))
		return 0
	}
	return must.ConvertToUInt64(len(b))
}
