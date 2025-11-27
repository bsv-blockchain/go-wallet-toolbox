package chaintracks

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracks/ingest"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracks/internal"
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
	if presentHeight <= liveHeightThreshold {
		bm.logger.Info("Skipping bulk synchronization - present height below live height threshold", slog.Any("present_height", presentHeight), slog.Any("live_height_threshold", liveHeightThreshold))
		return nil
	}

	bm.logger.Info("Starting bulk synchronization", slog.Any("present_height", presentHeight), slog.Any("initial_ranges", initialRanges))

	missingRange, err := models.NewHeightRange(0, presentHeight-liveHeightThreshold).Subtract(initialRanges.Bulk)
	if err != nil {
		return fmt.Errorf("failed to compute missing bulk range: %w", err)
	}

	for _, ingestor := range bm.bulkIngestors {
		if missingRange.IsEmpty() {
			break
		}

		bulkChunks, downloader, err := ingestor.Ingestor.Synchronize(ctx, presentHeight, missingRange)
		if err != nil {
			bm.logger.Error("Chaintracks service - error during bulk synchronization", slog.String("ingestor_name", ingestor.Name), slog.String("error", err.Error()))
			return fmt.Errorf("bulk synchronization failed for ingestor %s: %w", ingestor.Name, err)
		}

		if err := bm.processBulkChunks(ctx, bulkChunks, downloader, missingRange.MaxHeight); err != nil {
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

func (bm *bulkManager) LastHeader() (*wdk.ChainBlockHeader, *internal.ChainWork, error) {
	bm.locker.RLock()
	defer bm.locker.RUnlock()

	return bm.container.LastHeader()
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

func (bm *bulkManager) MigrateFromLiveHeaders(ctx context.Context, liveHeaders []*models.LiveBlockHeader) error {
	bm.locker.Lock()
	defer bm.locker.Unlock()

	// create data slice
	data := make([]byte, 0, len(liveHeaders)*80)
	for _, header := range liveHeaders {
		headerBytes, err := header.Bytes()
		if err != nil {
			return fmt.Errorf("failed to convert live header at height %d to bytes: %w", header.Height, err)
		}
		data = append(data, headerBytes...)
	}

	// add to container
	heightRange := models.NewHeightRange(liveHeaders[0].Height, liveHeaders[len(liveHeaders)-1].Height)
	err := bm.container.Add(ctx, data, heightRange)
	if err != nil {
		return fmt.Errorf("failed to add live headers to bulk container: %w", err)
	}

	return nil
}

func (bm *bulkManager) processBulkChunks(ctx context.Context, bulkChunks []ingest.BulkHeaderMinimumInfo, downloader ingest.BulkFileDownloader, maxHeight uint) error {
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

		dataRange := fileData.info.ToHeightRange()
		if dataRange.MaxHeight > maxHeight {
			dataRange.MaxHeight = maxHeight
		}

		if err := bm.container.Add(ctx, fileData.data, dataRange); err != nil {
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

func (bm *bulkManager) GetGapHeadersAsLive(ctx context.Context, presentHeight uint, liveInitialRange models.HeightRange) ([]wdk.ChainBlockHeader, error) {
	var newLiveHeaders []wdk.ChainBlockHeader
	maxBulkHeight := bm.GetHeightRange().MaxHeight
	minLiveHeight := presentHeight
	if liveInitialRange.NotEmpty() {
		minLiveHeight = liveInitialRange.MinHeight
	}

	if minLiveHeight <= maxBulkHeight || minLiveHeight < maxBulkHeight+addLiveRecursionLimit {
		// no gap to fill
		return nil, nil
	}

	// use bulk ingestors to fill the gap and treat fetched headers as live headers
	missingRange := models.NewHeightRange(maxBulkHeight+1, minLiveHeight-1)

	for _, ingestor := range bm.bulkIngestors {
		if missingRange.IsEmpty() {
			break
		}

		bulkChunks, downloader, err := ingestor.Ingestor.Synchronize(ctx, presentHeight, missingRange)
		if err != nil {
			bm.logger.Error("Chaintracks service - error during bulk synchronization to fill gap", slog.String("ingestor_name", ingestor.Name), slog.String("error", err.Error()))
			return nil, fmt.Errorf("bulk synchronization to fill gap failed for ingestor %s: %w", ingestor.Name, err)
		}

		for _, chunk := range bulkChunks {
			intersection := chunk.ToHeightRange().Intersect(missingRange)
			if intersection.IsEmpty() {
				continue
			}

			data, err := downloader(ctx, chunk)
			if err != nil {
				return nil, fmt.Errorf("failed to download bulk file %v to fill gap: %w", chunk, err)
			}

			if err := chunk.Validate(data); err != nil {
				return nil, fmt.Errorf("downloaded bulk file %v to fill gap is invalid: %w", chunk, err)
			}

			minIndex := intersection.MinHeight - chunk.FirstHeight
			maxIndex := intersection.MaxHeight - chunk.FirstHeight

			for i := minIndex; i <= maxIndex; i++ {
				startByte := i * 80
				endByte := startByte + 80
				headerData := data[startByte:endByte]

				baseHeader, err := wdk.ChainBaseBlockHeaderFromBytes(headerData)
				if err != nil {
					return nil, fmt.Errorf("failed to parse block header at height %d from bulk file %v: %w", chunk.FirstHeight+i, chunk, err)
				}

				blockHash, err := baseHeader.CalculateHash()
				if err != nil {
					return nil, fmt.Errorf("failed to compute hash for block header at height %d from bulk file %v: %w", chunk.FirstHeight+i, chunk, err)
				}

				header := wdk.ChainBlockHeader{
					ChainBaseBlockHeader: *baseHeader,
					Height:               chunk.FirstHeight + i,
					Hash:                 blockHash.String(),
				}

				newLiveHeaders = append(newLiveHeaders, header)
			}

			missingRange, err = missingRange.Subtract(intersection)
			if err != nil {
				return nil, fmt.Errorf("failed to compute missing range after filling gap with ingestor %s: %w", ingestor.Name, err)
			}
		}
	}

	return newLiveHeaders, nil
}
