package chaintracks

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracks/ingest"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracks/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

const bulkChunkSize = 100000

type bulkManager struct {
	logger        *slog.Logger
	bulkIngestors []NamedBulkIngestor

	locker    sync.RWMutex
	container *bulkHeadersContainer
}

func newBulkManager(logger *slog.Logger, bulkIngestors []NamedBulkIngestor, chain defs.BSVNetwork) *bulkManager {
	logger = logging.Child(logger, "chaintracks_bulk_manager")
	return &bulkManager{
		logger:        logger,
		bulkIngestors: bulkIngestors,
		container:     newBulkHeadersContainer(logger, bulkChunkSize, chain),
	}
}

func (bm *bulkManager) SyncBulkStorage(ctx context.Context, presentHeight uint, initialRanges models.HeightRanges) error {
	bm.logger.Info("Starting bulk synchronization", slog.Any("present_height", presentHeight), slog.Any("initial_ranges", initialRanges))

	missingRange := models.NewHeightRange(0, presentHeight)
	for _, ingestor := range bm.bulkIngestors {
		if missingRange.IsEmpty() {
			break
		}

		bulkChunks, downloader, err := ingestor.Ingestor.Synchronize(ctx, presentHeight, missingRange)
		if err != nil {
			bm.logger.Error("Chaintracks service - error during bulk synchronization", slog.String("ingestor_name", ingestor.Name), slog.String("error", err.Error()))
			return fmt.Errorf("bulk synchronization failed for ingestor %s: %w", ingestor.Name, err)
		}

		if err := bm.processBulkChunks(ctx, bulkChunks, downloader); err != nil {
			return fmt.Errorf("failed to process bulk chunks from ingestor %s: %w", ingestor.Name, err)
		}

		missingRange, err = missingRange.Subtract(bm.GetHeightRange())
		if err != nil {
			return fmt.Errorf("failed to compute missing range after processing ingestor %s: %w", ingestor.Name, err)
		}
	}

	bm.locker.Lock()
	defer bm.locker.Unlock()
	if err := bm.container.Update(); err != nil {
		return fmt.Errorf("failed to update bulk headers container: %w", err)
	}

	return nil
}

func (bm *bulkManager) GetHeightRange() models.HeightRange {
	bm.locker.RLock()
	defer bm.locker.RUnlock()

	return bm.container.Range()
}

func (bm *bulkManager) FindHeaderForHeight(height uint) (*wdk.ChainBlockHeader, error) {
	bm.locker.RLock()
	defer bm.locker.RUnlock()

	return bm.container.FindHeaderForHeight(height)
}

func (bm *bulkManager) FilesInfo() *ingest.BulkHeaderFilesInfo {
	bm.locker.RLock()
	defer bm.locker.RUnlock()

	return &ingest.BulkHeaderFilesInfo{
		HeadersPerFile: bulkChunkSize,
		Files:          bm.container.FilesInfo(),
	}
}

func (bm *bulkManager) GetFileDataByIndex(fileID int) (*ingest.BulkFileData, error) {
	bm.locker.RLock()
	defer bm.locker.RUnlock()

	return bm.container.GetFileDataByIndex(fileID)
}

func (bm *bulkManager) processBulkChunks(ctx context.Context, bulkChunks []ingest.BulkHeaderMinimumInfo, downloader ingest.BulkFileDownloader) error {
	chunksToLoad := bm.getChunksToLoad(bulkChunks)
	type chunkWithInfo struct {
		data []byte
		info ingest.BulkHeaderMinimumInfo
	}
	loadedChunks := make([]chunkWithInfo, 0, len(chunksToLoad))

	for _, chunk := range chunksToLoad {
		bm.logger.Info("Downloading bulk file", slog.Any("bulk_info", chunk))
		data, err := downloader(ctx, chunk)
		if err != nil {
			return fmt.Errorf("failed to download bulk file %v: %w", chunk, err)
		}

		if err := chunk.Validate(data); err != nil {
			return fmt.Errorf("downloaded bulk file %v is invalid: %w", chunk, err)
		}
		loadedChunks = append(loadedChunks, chunkWithInfo{
			data: data,
			info: chunk,
		})
	}

	bm.locker.Lock()
	defer bm.locker.Unlock()

	for _, fileData := range loadedChunks {
		if !bm.shouldAddNewFile(&fileData.info) {
			// NOTE: If another goroutine added the same bulk file while we were downloading, we skip adding it again
			continue
		}

		if err := bm.container.Add(ctx, fileData.data, fileData.info.ToHeightRange()); err != nil {
			return fmt.Errorf("failed to add bulk file %v to container: %w", fileData.info, err)
		}
	}

	return nil
}

func (bm *bulkManager) getChunksToLoad(chunks []ingest.BulkHeaderMinimumInfo) []ingest.BulkHeaderMinimumInfo {
	bm.locker.RLock()
	defer bm.locker.RUnlock()

	filteredChunks := make([]ingest.BulkHeaderMinimumInfo, 0)
	for _, chunk := range chunks {
		if bm.shouldAddNewFile(&chunk) {
			filteredChunks = append(filteredChunks, chunk)
		}
	}

	return filteredChunks
}

func (bm *bulkManager) shouldAddNewFile(info *ingest.BulkHeaderMinimumInfo) bool {
	currentRange := bm.container.Range()
	rangeToAdd := info.ToHeightRange().Above(currentRange)
	return !rangeToAdd.IsEmpty()
}
