package ingest

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracks/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/httpx"
)

// BulkIngestorCDN provides bulk ingestion of blockchain headers from a CDN source for a specific BSV network.
// It manages downloading and synchronizing block header files using the configured CDNReader and source URL.
// The ingestor is initialized with a logger, targeted BSV network, and source URL for the CDN.
type BulkIngestorCDN struct {
	logger    *slog.Logger
	reader    *CDNReader
	chain     defs.BSVNetwork
	sourceURL string
}

// NewBulkIngestorCDN creates a new BulkIngestorCDN for ingesting block headers from a CDN source using the given config.
func NewBulkIngestorCDN(logger *slog.Logger, chain defs.BSVNetwork, config defs.CDNBulkIngestorConfig) *BulkIngestorCDN {
	logger = logging.Child(logger, "bulk_ingestor_cdn")
	return &BulkIngestorCDN{
		logger:    logger,
		chain:     chain,
		reader:    NewCDNReader(logger, config.SourceURL, httpx.NewRestyClientFactory().New()),
		sourceURL: config.SourceURL,
	}
}

// BulkFileDownloader is a function type that downloads and returns bulk block header file data with metadata.
// It takes a context and BulkHeaderFileInfo as input, returning BulkFileData and an error if the download fails.
type BulkFileDownloader = func(ctx context.Context, fileInfo BulkHeaderFileInfo) (BulkFileData, error)

// Synchronize retrieves available bulk header files for the configured BSV network and prepares chunks for ingestion.
// It validates file metadata, checks network consistency, and returns a list of chunked header information for sync.
func (b *BulkIngestorCDN) Synchronize(ctx context.Context, presentHeight uint, ranges models.HeightRanges) ([]BulkHeaderFileInfo, BulkFileDownloader, error) {
	// TODO: PresentHeight and ranges are not used in TS implementation, consider using them for optimization

	filesInfo, err := b.reader.FetchBulkHeaderFilesInfo(ctx, b.chain)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch bulk header files info: %w", err)
	}

	for _, file := range filesInfo.Files {
		b.logger.Info("Found bulk header file",
			slog.String("file_name", file.FileName),
			logging.Number("start_height", file.FirstHeight),
			logging.Number("end_height", file.Count))

		if len(file.FileHash) == 0 {
			return nil, nil, fmt.Errorf("file %s is missing hash", file.FileName)
		}

		file.Chain, err = defs.ParseBSVNetworkStr(string(file.Chain))
		if err != nil {
			return nil, nil, fmt.Errorf("file %s has invalid chain: %w", file.FileName, err)
		}

		if file.Chain != b.chain {
			return nil, nil, fmt.Errorf("file %s has mismatched chain: expected %s, got %s", file.FileName, b.chain, file.Chain)
		}

		file.SourceURL = &b.sourceURL
	}

	return filesInfo.Files, b.bulkFileDownloader(), nil
}

func (b *BulkIngestorCDN) bulkFileDownloader() BulkFileDownloader {
	return func(ctx context.Context, fileInfo BulkHeaderFileInfo) (BulkFileData, error) {
		b.logger.Info("Downloading bulk header file", slog.String("file_name", fileInfo.FileName))

		bulkFile, err := b.reader.FetchBulkHeaderFile(ctx, fileInfo)
		if err != nil {
			return BulkFileData{}, fmt.Errorf("failed to download bulk header file %s: %w", fileInfo.FileName, err)
		}

		return *bulkFile, nil
	}
}
