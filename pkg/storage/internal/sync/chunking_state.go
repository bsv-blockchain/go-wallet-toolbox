package sync

import (
	"iter"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type chunkingState struct {
	args *wdk.RequestSyncChunkArgs

	itemsCounter     uint64
	prevItemsCounter uint64
	roughSize        uint64
}

func newChunkingState(args *wdk.RequestSyncChunkArgs) *chunkingState {
	return &chunkingState{
		args: args,
	}
}

func (s *chunkingState) update(num uint64, roughSize uint64) {
	s.prevItemsCounter = s.itemsCounter
	s.itemsCounter += num
	s.roughSize = roughSize
}

func (s *chunkingState) doWhileChunkProcessed(page *queryopts.Paging) iter.Seq[*queryopts.Paging] {
	return func(yield func(*queryopts.Paging) bool) {
		if !yield(page) {
			return
		}

		for !s.chunkProcessed() {
			page.Next()
			if !yield(page) {
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

func (s *chunkingState) freeSlots() uint64 {
	if s.args.MaxItems == 0 || s.itemsCounter >= s.args.MaxItems {
		return 0
	}
	return s.args.MaxItems - s.itemsCounter
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
