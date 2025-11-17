package chaintracks

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracks/ingest"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracks/models"
	"github.com/go-softwarelab/common/pkg/must"
)

type bulkFileData struct {
	ingest.BulkFileData
}

type bulkManager struct {
	logger        *slog.Logger
	bulkIngestors []NamedBulkIngestor

	locker    sync.RWMutex
	bulkFiles []bulkFileData
}

func newBulkManager(logger *slog.Logger, bulkIngestors []NamedBulkIngestor) *bulkManager {
	logger = logging.Child(logger, "chaintracks_bulk_manager")
	return &bulkManager{
		logger:        logger,
		bulkIngestors: bulkIngestors,
	}
}

func (bm *bulkManager) SyncBulkStorage(ctx context.Context, presentHeight uint, initialRanges models.HeightRanges) (err error) {
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

		providedRange := models.NewEmptyHeightRange()
		for _, chunk := range bulkChunks {
			providedRange, err = providedRange.Union(chunk.ToHeightRange())
			if err != nil {
				return fmt.Errorf("failed to compute provided height range from ingestor %s: %w", ingestor.Name, err)
			}
		}

		missingRange, err = missingRange.Subtract(providedRange)
		if err != nil {
			return fmt.Errorf("failed to compute missing height range after ingestor %s: %w", ingestor.Name, err)
		}

		// TODO: Implement DONE check and break if done
	}

	return nil
}

func (bm *bulkManager) GetHeightRange() models.HeightRange {
	bm.locker.RLock()
	defer bm.locker.RUnlock()

	if len(bm.bulkFiles) == 0 {
		return models.NewEmptyHeightRange()
	}
	first := bm.bulkFiles[0]
	last := bm.bulkFiles[len(bm.bulkFiles)-1]

	minHeight := first.Info.FirstHeight
	maxHeight := last.Info.FirstHeight + must.ConvertToUInt(last.Info.Count) - 1

	return models.NewHeightRange(minHeight, maxHeight)
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
		if bm.alreadyContainsIdentical(&fileData.Info) {
			// NOTE: If another goroutine added the same bulk file while we were downloading, we skip adding it again
			continue
		}

		bm.bulkFiles = append(bm.bulkFiles, bulkFileData{BulkFileData: fileData})
	}

	return nil
}

func (bm *bulkManager) getChunksToLoad(chunks []ingest.BulkHeaderFileInfo) []ingest.BulkHeaderFileInfo {
	bm.locker.RLock()
	defer bm.locker.RUnlock()

	filteredChunks := make([]ingest.BulkHeaderFileInfo, 0)
	for _, chunk := range chunks {
		if !bm.alreadyContainsIdentical(&chunk) {
			filteredChunks = append(filteredChunks, chunk)
		}
	}

	return filteredChunks
}

func (bm *bulkManager) alreadyContainsIdentical(newBulk *ingest.BulkHeaderFileInfo) bool {
	for _, existingBulk := range bm.bulkFiles {
		if existingBulk.Info.Equals(newBulk) {
			return true
		}
	}

	return false
}
