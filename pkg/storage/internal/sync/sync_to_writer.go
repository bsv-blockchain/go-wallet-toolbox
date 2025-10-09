package sync

import (
	"context"
	"fmt"
	"iter"
	"log/slog"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/to"
)

type ReaderToWriter struct {
	logger *slog.Logger
}

func NewReaderToWriter(logger *slog.Logger) *ReaderToWriter {
	return &ReaderToWriter{
		logger: logging.Child(logger, "ReaderToWriter"),
	}
}

func (s *ReaderToWriter) Sync(
	ctx context.Context,
	auth wdk.AuthID,
	reader, writer wdk.WalletStorageProvider,
	maxSyncChunkSize, maxSyncItems uint64,
) (inserts, updates int, err error) {
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

	if writerSettings.StorageIdentityKey == readerSettings.StorageIdentityKey {
		return 0, 0, fmt.Errorf("cannot sync to the same storage: %s", writerSettings.StorageIdentityKey)
	}

	userIdentityKey := auth.IdentityKey
	userOnWriterSide, err := writer.FindOrInsertUser(ctx, userIdentityKey)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to find or insert user in writer storage: %w", err)
	}

	userAuthOnWriterSide := wdk.AuthID{
		IdentityKey: userIdentityKey,
		UserID:      to.Ptr(userOnWriterSide.User.UserID),
	}

	logger := s.logger.With(
		slog.String("to_storage", writerSettings.StorageIdentityKey),
		slog.String("from_storage", readerSettings.StorageIdentityKey),
		slog.String("user_identity_key", userIdentityKey),
	)
	logger.Info("beginning sync")

	var state syncingState

	for range state.doWhileChangesMade() {
		writerSyncState, err := writer.FindOrInsertSyncStateAuth(ctx, userAuthOnWriterSide, readerSettings.StorageIdentityKey, readerSettings.StorageName)
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
			IdentityKey:            userIdentityKey,

			Since:        syncState.When,
			MaxRoughSize: maxSyncChunkSize,
			MaxItems:     maxSyncItems,
			Offsets:      s.buildOffsets(syncMap),
		}

		logger.Debug("getting sync chunk from reader", slog.Any("getSyncChunkArgs", getSyncChunkArgs))

		chunk, err := reader.GetSyncChunk(ctx, getSyncChunkArgs)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to get sync chunk from reader storage: %w", err)
		}

		logger.Debug("processing sync chunk in writer")

		processChunkResult, err := writer.ProcessSyncChunk(ctx, getSyncChunkArgs, chunk)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to process sync chunk in writer storage: %w", err)
		}

		logger.Info("processed sync chunk", slog.Int("inserts", processChunkResult.Inserts), slog.Int("updates", processChunkResult.Updates))
		state.updateState(processChunkResult.Inserts, processChunkResult.Updates)

		if processChunkResult.Done {
			logger.Info("Writer reports done processing sync chunk")
			break
		}
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
