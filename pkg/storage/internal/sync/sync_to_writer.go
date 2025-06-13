package sync

import (
	"context"
	"fmt"
	"iter"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
)

const (
	maxSyncChunkSize = 10_000_000 // ~10 MB
	maxSyncItems     = 1000
)

type ReaderToWriter struct{}

func NewReaderToWriter() *ReaderToWriter {
	return &ReaderToWriter{}
}

func (s *ReaderToWriter) Sync(ctx context.Context, auth wdk.AuthID, reader, writer wdk.WalletStorageProvider) (inserts, updates int, err error) {
	writerSettings, err := writer.MakeAvailable(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to make writer storage available: %w", err)
	}

	readerSettings, err := reader.MakeAvailable(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to make reader storage available: %w", err)
	}

	if writerSettings.Chain != readerSettings.Chain {
		return 0, 0, fmt.Errorf("cannot sync between different chains: reader chain %s, writer chain %s", readerSettings.Chain, writerSettings.Chain)
	}

	var state syncingState

	for range state.doWhileChangesMade() {
		writerSyncState, err := writer.FindOrInsertSyncStateAuth(ctx, auth, readerSettings.StorageIdentityKey, readerSettings.StorageName)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to find or insert sync state auth: %w", err)
		}

		syncState := writerSyncState.SyncState

		syncMap, err := wdk.NewSyncMapFromJSON([]byte(syncState.SyncMap))
		if err != nil {
			return 0, 0, fmt.Errorf("failed to parse sync map: %w", err)
		}

		getSyncChunkArgs := wdk.RequestSyncChunkArgs{
			FromStorageIdentityKey: readerSettings.StorageIdentityKey,
			ToStorageIdentityKey:   writerSettings.StorageIdentityKey,
			IdentityKey:            auth.IdentityKey,

			Since:        syncState.When,
			MaxRoughSize: maxSyncChunkSize,
			MaxItems:     maxSyncItems,
			Offsets:      s.buildOffsets(syncMap),
		}

		chunk, err := reader.GetSyncChunk(ctx, getSyncChunkArgs)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to get sync chunk from reader storage: %w", err)
		}

		processChunkResult, err := writer.ProcessSyncChunk(ctx, getSyncChunkArgs, chunk)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to process sync chunk in writer storage: %w", err)
		}

		state.updateState(processChunkResult.Inserts, processChunkResult.Updates)

		if processChunkResult.Done {
			break
		}

		// TODO Log the sync chunk received (it needs the Manager to include a logger)

	}
	return state.inserts, state.updates, nil
}

func (s *ReaderToWriter) buildOffsets(syncMap wdk.SyncMap) []wdk.SyncOffsets {
	offsets := make([]wdk.SyncOffsets, 0, len(wdk.AllEntityNames))
	for _, entityName := range wdk.AllEntityNames {
		syncMapEntity, ok := syncMap[entityName]
		if !ok {
			continue
		}

		offsets = append(offsets, wdk.SyncOffsets{
			Name:   entityName,
			Offset: syncMapEntity.Count,
		})
	}
	return offsets
}

type syncingState struct {
	updates               int
	inserts               int
	nothingChangedCounter int
}

func (s *syncingState) updateState(inserts, updates int) {
	s.inserts += inserts
	s.updates += updates

	// NOTE: Depends on storage provider implementation,
	// ProcessSyncChunk may need to process one more chunk after the empty one, and then returns Done = true.
	// But if not, this logic will ensure we don't loop unnecessarily.
	if updates == 0 && inserts == 0 {
		s.nothingChangedCounter++
	} else {
		s.nothingChangedCounter = 0
	}
}

func (s *syncingState) doWhileChangesMade() iter.Seq[int] {
	const safetyLimit = 1000    // Safety limit to prevent infinite loops
	const maxNothingChanged = 2 // Allow at most 2 consecutive empty chunks
	return func(yield func(int) bool) {
		if !yield(0) {
			return
		}
		for i := 1; i <= safetyLimit && s.nothingChangedCounter < maxNothingChanged; i++ {
			if !yield(i) {
				return
			}
		}
	}
}
