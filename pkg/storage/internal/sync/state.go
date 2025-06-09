package sync

import (
	"context"
	"fmt"
	"iter"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
)

type chunkingState struct {
	ctx              context.Context
	itemsCounter     uint64
	prevItemsCounter uint64
	err              error
	roughSize        uint64
	args             *wdk.RequestSyncChunkArgs
}

func newChunkingState(ctx context.Context, args *wdk.RequestSyncChunkArgs) *chunkingState {
	return &chunkingState{
		ctx:  ctx,
		args: args,
	}
}

func (s *chunkingState) update(num uint64, roughSize uint64) {
	s.prevItemsCounter = s.itemsCounter
	s.itemsCounter += num
	s.roughSize = roughSize
}

func (s *chunkingState) doWhileChunkProcessed() iter.Seq[struct{}] {
	return func(yield func(struct{}) bool) {
		if !yield(struct{}{}) {
			return
		}

		for !s.chunkProcessed() {
			if err := s.ctx.Err(); err != nil {
				s.err = fmt.Errorf("context canceled, aborting: %w", err)
				return
			}

			if !yield(struct{}{}) {
				return
			}
		}
	}
}

func (s *chunkingState) chunkProcessed() bool {
	return s.prevItemsCounter == s.itemsCounter || s.reachedMax()
}

func (s *chunkingState) reachedMax() bool {
	return s.itemsCounter >= s.args.MaxItems || s.roughSize >= s.args.MaxRoughSize
}

func (s *chunkingState) getNextChunkerUntilReachedMax(chunks iter.Seq[Chunker]) iter.Seq[Chunker] {
	return func(yield func(Chunker) bool) {
		for chunker := range chunks {
			if s.reachedMax() {
				return
			}

			if !yield(chunker) {
				return
			}
		}
	}
}
