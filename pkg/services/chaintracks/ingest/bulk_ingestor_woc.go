package ingest

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracks/models"
	"github.com/go-softwarelab/common/pkg/must"
	"github.com/go-softwarelab/common/pkg/to"
)

type BulkIngestorWOC struct {
	logger    *slog.Logger
	chain     defs.BSVNetwork
	wocClient *wocClient
}

func NewBulkIngestorWOC(logger *slog.Logger, chain defs.BSVNetwork, opts ...func(options *BulkIngestorWocOptions)) *BulkIngestorWOC {
	logger = logging.Child(logger, "bulk_ingestor_woc")

	options := to.OptionsWithDefault(DefaultBulkIngestorWocOptions(), opts...)

	return &BulkIngestorWOC{
		logger:    logger,
		chain:     chain,
		wocClient: newWocClient(logger, chain, options.APIKey, options.RestyClientFactory.New()),
	}
}

func (b *BulkIngestorWOC) Synchronize(ctx context.Context, presentHeight uint, rangeToFetch models.HeightRange) ([]BulkHeaderFileInfo, BulkFileDownloader, error) {
	allFiles, err := b.fetchBulkHeaderFilesInfo(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch bulk header files info: %w", err)
	}

	if len(allFiles) == 0 {
		return nil, nil, fmt.Errorf("no bulk header files available from WhatsOnChain")
	}

	neededFiles := make([]wocBulkFileInfo, 0)
	for _, file := range allFiles {
		if file.heightRange.Overlaps(rangeToFetch) {
			neededFiles = append(neededFiles, file)
		}
	}

	result := make([]BulkHeaderFileInfo, 0, len(neededFiles))
	for _, file := range neededFiles {
		bulkFileInfo, err := b.toBulkHeaderFileInfo(ctx, &file)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to convert to BulkHeaderFileInfo for file %s: %w", file.filename, err)
		}

		result = append(result, *bulkFileInfo)
	}

	return result, b.bulkFileDownloader(), nil

}

func (b *BulkIngestorWOC) toBulkHeaderFileInfo(ctx context.Context, file *wocBulkFileInfo) (*BulkHeaderFileInfo, error) {
	prevChainWork := prevChainWorkForGenesis
	prevHash := genesisAsPrevBlockHash
	if file.heightRange.MinHeight > 0 {
		prevBlock, err := b.wocClient.GetBlockByHeight(ctx, file.heightRange.MinHeight-1)
		if err != nil {
			return nil, fmt.Errorf("failed to get previous block at height %d: %w", file.heightRange.MinHeight-1, err)
		}

		prevChainWork = prevBlock.Chainwork
		prevHash = prevBlock.Hash
	}

	lastBlock, err := b.wocClient.GetBlockByHeight(ctx, file.heightRange.MaxHeight)
	if err != nil {
		return nil, fmt.Errorf("failed to get last block at height %d: %w", file.heightRange.MaxHeight, err)
	}

	return &BulkHeaderFileInfo{
		FileName:    fmt.Sprintf("%d_%d_headers.bin", file.heightRange.MinHeight, file.heightRange.MaxHeight),
		FirstHeight: file.heightRange.MinHeight,
		Count:       must.ConvertToIntFromUnsigned(file.heightRange.MaxHeight) - must.ConvertToIntFromUnsigned(file.heightRange.MinHeight) + 1,
		Chain:       b.chain,
		SourceURL:   to.Ptr(file.url),

		PrevChainWork: prevChainWork,
		PrevHash:      prevHash,

		LastChainWork: lastBlock.Chainwork,
		LastHash:      &lastBlock.Hash,

		// Not supported, we don't download the file at this point and WoC doesn't provide it in metadata
		FileHash: nil,
	}, nil
}

func (b *BulkIngestorWOC) bulkFileDownloader() BulkFileDownloader {
	return func(ctx context.Context, fileInfo BulkHeaderFileInfo) (BulkFileData, error) {
		if fileInfo.SourceURL == nil {
			panic("SourceURL is nil in bulk file downloader")
		}

		b.logger.Info("Downloading bulk header file", slog.String("file_name", fileInfo.FileName))

		content, err := b.wocClient.DownloadHeaderFile(ctx, *fileInfo.SourceURL)
		if err != nil {
			return BulkFileData{}, fmt.Errorf("failed to download bulk header file %s: %w", fileInfo.FileName, err)
		}

		return BulkFileData{
			Info:       fileInfo,
			Data:       content,
			AccessedAt: time.Now(),
		}, nil
	}
}

type wocBulkFileInfo struct {
	heightRange models.HeightRange
	url         string
	filename    string
}

func (b *BulkIngestorWOC) fetchBulkHeaderFilesInfo(ctx context.Context) ([]wocBulkFileInfo, error) {
	response, err := b.wocClient.GetHeadersResourceList(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get headers resource list from WhatsOnChain: %w", err)
	}

	result := make([]wocBulkFileInfo, 0, len(response.Files))
	for _, fileURL := range response.Files {
		filename, heightRange, err := b.parseURL(ctx, fileURL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse height range from URL %s: %w", fileURL, err)
		}

		result = append(result, wocBulkFileInfo{
			heightRange: heightRange,
			url:         fileURL,
			filename:    filename,
		})
	}

	return result, nil
}

// parseURL parses the height range from the given WhatsOnChain bulk header file URL.
// "https://api.whatsonchain.com/v1/bsv/main/block/headers/0_10000_headers.bin",
// "https://api.whatsonchain.com/v1/bsv/main/block/headers/10001_20000_headers.bin",
// (...)
// "https://api.whatsonchain.com/v1/bsv/main/block/headers/latest"
// The latest endpoint - we don't know the max height by URL alone; the min height is previous max + 1
// So we need to get the Content-Disposition header from the HEAD request to get the actual filename
func (b *BulkIngestorWOC) parseURL(ctx context.Context, url string) (filename string, heightRange models.HeightRange, err error) {
	parts := strings.Split(url, "/block/headers/")
	if len(parts) != 2 {
		err = fmt.Errorf("invalid URL format: %s", url)
		return
	}
	filename = parts[1]

	if filename == "latest" {
		filename, err = b.getLatestHeightRange(ctx, url)
		if err != nil {
			err = fmt.Errorf("failed to get latest height range from URL %s: %w", url, err)
			return
		}
	}

	_, err = fmt.Sscanf(filename, "%d_%d_headers.bin", &heightRange.MinHeight, &heightRange.MaxHeight)
	if err != nil {
		err = fmt.Errorf("failed to parse height range from filename %s: %w", filename, err)
		return
	}

	return
}

// getRedirectURLForLatest doesn't follow redirects, instead it fetches the Location header from the 302 response
// this is needed to get the height range from the redirected URL
func (b *BulkIngestorWOC) getLatestHeightRange(ctx context.Context, latestURL string) (string, error) {
	contentHeader, err := b.wocClient.GetContentDispositionFilename(ctx, latestURL)
	if err != nil {
		return "", fmt.Errorf("failed to get Content-Disposition header from WhatsOnChain: %w", err)
	}

	// example: Content-Disposition: attachment; filename=922001_923532_headers.bin
	var filename string
	if _, err = fmt.Sscanf(contentHeader, "attachment; filename=%s", &filename); err != nil {
		return "", fmt.Errorf("failed to parse filename from Content-Disposition header: %w", err)
	}

	return filename, nil
}
