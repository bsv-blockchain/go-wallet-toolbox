package chaintracks

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

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

func newBulkManager(logger *slog.Logger, bulkIngestors []NamedBulkIngestor) *bulkManager {
	logger = logging.Child(logger, "chaintracks_bulk_manager")
	return &bulkManager{
		logger:        logger,
		bulkIngestors: bulkIngestors,
		container:     newBulkHeadersContainer(logger, bulkChunkSize),
	}
}

func (bm *bulkManager) SyncBulkStorage(ctx context.Context, presentHeight uint, initialRanges models.HeightRanges) error {
	bm.logger.Info("Starting bulk synchronization", slog.Any("present_height", presentHeight), slog.Any("initial_ranges", initialRanges))

	// current_range="[0 - 915511]" data_range="[916001 - 918000]"
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

func (bm *bulkManager) processBulkChunks(ctx context.Context, bulkChunks []ingest.BulkHeaderFileInfo, downloader ingest.BulkFileDownloader) error {
	chunksToLoad := bm.getChunksToLoad(bulkChunks)
	loadedChunks := make([]ingest.BulkFileData, 0, len(chunksToLoad))

	for _, chunk := range chunksToLoad {
		bm.logger.Info("Downloading bulk file", slog.Any("bulk_info", chunk))
		fileData, err := downloader(ctx, chunk)
		if err != nil {
			return fmt.Errorf("failed to download bulk file %v: %w", chunk, err)
		}

		if err := fileData.Validate(); err != nil {
			return fmt.Errorf("downloaded bulk file %v is invalid: %w", chunk, err)
		}
		loadedChunks = append(loadedChunks, fileData)
	}

	bm.locker.Lock()
	defer bm.locker.Unlock()

	for _, fileData := range loadedChunks {
		if !bm.shouldAddNewFile(&fileData.Info) {
			// NOTE: If another goroutine added the same bulk file while we were downloading, we skip adding it again
			continue
		}

		if err := bm.container.Add(ctx, fileData.Data, fileData.Info.ToHeightRange()); err != nil {
			return fmt.Errorf("failed to add bulk file %v to container: %w", fileData.Info, err)
		}
	}

	return nil
}

func (bm *bulkManager) getChunksToLoad(chunks []ingest.BulkHeaderFileInfo) []ingest.BulkHeaderFileInfo {
	bm.locker.RLock()
	defer bm.locker.RUnlock()

	filteredChunks := make([]ingest.BulkHeaderFileInfo, 0)
	for _, chunk := range chunks {
		if bm.shouldAddNewFile(&chunk) {
			filteredChunks = append(filteredChunks, chunk)
		}
	}

	return filteredChunks
}

func (bm *bulkManager) shouldAddNewFile(info *ingest.BulkHeaderFileInfo) bool {
	currentRange :=  bm.container.Range()
	rangeToAdd := info.ToHeightRange().Above(currentRange)
	return !rangeToAdd.IsEmpty()
}
