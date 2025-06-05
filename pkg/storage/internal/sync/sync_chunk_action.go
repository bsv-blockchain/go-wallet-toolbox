package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"log/slog"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/logging"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/seq"
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
	var num int
	var err error

	itemsCounter := 0

	offsetsLookup := s.makeOffsetsLookup(args)

	for _, chunker := range s.chunkers {
		if !chunker.IsApplicable(offsetsLookup) {
			continue
		}

		relativeOffset := 0
		for range s.safeLoopIterator(1000) {
			if err = ctx.Err(); err != nil {
				return fmt.Errorf("context canceled, aborting: %w", err)
			}

			freeSlots := args.MaxItems - itemsCounter
			limit := min(freeSlots, max(10, args.MaxItems/chunker.MaxDivider()))

			num, err = chunker.Process(ctx, userID, limit, relativeOffset, offsetsLookup, args.Since, result)
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

			relativeOffset += num
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

func (s *syncChunkAction) safeLoopIterator(maxIterations int) iter.Seq[int] {
	return seq.RangeWithStep(0, maxIterations, 1)
}

func (s *syncChunkAction) approxJSONSize(chunk *wdk.SyncChunk) int {
	// TODO it could be less precise and use a more efficient way to estimate size
	b, err := json.Marshal(chunk)
	if err != nil {
		return 0
	}
	return len(b)
}
