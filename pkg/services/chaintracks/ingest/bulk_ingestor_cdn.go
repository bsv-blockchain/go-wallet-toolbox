package ingest

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracks/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/httpx"
	"github.com/go-softwarelab/common/pkg/must"
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
type BulkFileDownloader = func(ctx context.Context, fileInfo BulkHeaderMinimumInfo) ([]byte, error)

// Synchronize retrieves available bulk header files for the configured BSV network and prepares chunks for ingestion.
// It validates file metadata, checks network consistency, and returns a list of chunked header information for sync.
func (b *BulkIngestorCDN) Synchronize(ctx context.Context, presentHeight uint, rangeToFetch models.HeightRange) ([]BulkHeaderMinimumInfo, BulkFileDownloader, error) {
	// TODO: PresentHeight and ranges are not used in TS implementation, consider using them for optimization

	filesInfo, err := b.reader.FetchBulkHeaderFilesInfo(ctx, b.chain)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch bulk header files info: %w", err)
	}

	bulkInfo := make([]BulkHeaderMinimumInfo, 0, len(filesInfo.Files))
	for i := range filesInfo.Files {
		file := &filesInfo.Files[i]
		b.logger.Info("Found bulk header file",
			slog.String("file_name", file.FileName),
			logging.Number("start_height", file.FirstHeight),
			logging.Number("end_height", must.ConvertToIntFromUnsigned(file.FirstHeight)+file.Count-1))

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

		file.SourceURL = b.sourceURL

		bulkInfo = append(bulkInfo, file.BulkHeaderMinimumInfo)
	}

	return bulkInfo, b.bulkFileDownloader(), nil
}

func (b *BulkIngestorCDN) bulkFileDownloader() BulkFileDownloader {
	return func(ctx context.Context, fileInfo BulkHeaderMinimumInfo) ([]byte, error) {
		b.logger.Info("Downloading bulk header file", slog.String("file_name", fileInfo.FileName))

		bulkFile, err := b.reader.DownloadBulkHeaderFile(ctx, fileInfo.FileName)
		if err != nil {
			return nil, fmt.Errorf("failed to download bulk header file %s: %w", fileInfo.FileName, err)
		}

		return bulkFile, nil
	}
}
