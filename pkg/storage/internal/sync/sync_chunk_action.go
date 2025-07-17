package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/must"
	"github.com/go-softwarelab/common/pkg/seq"
	"github.com/go-softwarelab/common/pkg/to"
)

type GetSyncChunkAction struct {
	logger   *slog.Logger
	repo     Repository
	chunkers []Chunker
	args     *wdk.RequestSyncChunkArgs
}

func NewGetSyncChunkAction(logger *slog.Logger, repo Repository, args *wdk.RequestSyncChunkArgs) *GetSyncChunkAction {
	return &GetSyncChunkAction{
		logger:   logging.Child(logger, "getSyncChunk"),
		repo:     repo,
		chunkers: all(repo),
		args:     args,
	}
}

func (s *GetSyncChunkAction) Get(ctx context.Context) (*wdk.SyncChunk, error) {
	user, err := s.repo.FindUser(ctx, s.args.IdentityKey)
	if err != nil {
		return nil, fmt.Errorf("cannot find user: %w", err)
	}

	if user == nil {
		return nil, fmt.Errorf("user with identity key %s not found", s.args.IdentityKey)
	}

	chunk := wdk.NewSyncChunk(
		s.args.FromStorageIdentityKey,
		s.args.ToStorageIdentityKey,
		s.args.IdentityKey,
	)

	if s.args.Since == nil || user.UpdatedAt.After(*s.args.Since) {
		chunk.User = user.ToWDK()
	}

	if err = s.process(ctx, user.ID, chunk); err != nil {
		return nil, fmt.Errorf("failed to process sync chunk: %w", err)
	}

	return chunk, nil
}

func (s *GetSyncChunkAction) process(ctx context.Context, userID int, result *wdk.SyncChunk) error {
	state := newChunkingState(s.args)

	offsetsLookup := s.makeOffsetsLookup()

	applicableChunkers := seq.Filter(seq.FromSlice(s.chunkers), func(chunker Chunker) bool {
		return chunker.IsApplicable(offsetsLookup)
	})

	for chunker := range state.getNextChunkerUntilReachedMax(applicableChunkers) {
		var firstPage = chunker.FirstPage(offsetsLookup)

		for page := range state.doWhileChunkProcessed(firstPage) {
			if err := ctx.Err(); err != nil {
				return fmt.Errorf("context canceled, aborting: %w", err)
			}

			limit := to.NoMoreThan(state.freeSlots(), chunker.MaxPageSize())
			page.Limit = must.ConvertToIntFromUnsigned(to.NoMoreThan(limit, maximumAvailablePageSize))

			num, err := chunker.Process(ctx, userID, page, s.args.Since, result)
			if err != nil {
				return fmt.Errorf("chunker %s failed: %w", chunker.Name(), err)
			}

			state.update(num, s.approxJSONSize(result))
		}
	}

	return nil
}

func (s *GetSyncChunkAction) makeOffsetsLookup() OffsetsLookup {
	offsetsLookup := make(OffsetsLookup, len(s.args.Offsets))
	for _, it := range s.args.Offsets {
		offsetsLookup[it.Name] = it.Offset
	}
	return offsetsLookup
}

func (s *GetSyncChunkAction) approxJSONSize(chunk *wdk.SyncChunk) uint64 {
	b, err := json.Marshal(chunk)
	if err != nil {
		s.logger.Warn("failed to marshal sync chunk for size estimation", slog.String("error", err.Error()))
		return 0
	}
	return must.ConvertToUInt64(len(b))
}
