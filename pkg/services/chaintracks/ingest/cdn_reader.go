package ingest

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/httpx"
	"github.com/go-resty/resty/v2"
)

// BabbageCDNBaseURL is the base URL for accessing block header files from the Project Babbage public CDN.
const (
	BabbageCDNBaseURL = "https://cdn.projectbabbage.com/blockheaders"
)

// CDNReader provides methods to interact with and retrieve blockchain header data from a remote CDN service.
type CDNReader struct {
	logger *slog.Logger
	resty  *resty.Client
}

// NewCDNReader creates a new CDNReader for fetching blockchain header data from a CDN using the specified logger and client.
// It configures a Resty client with custom headers, logging, and base URL for use in CDNReader operations.
// The returned CDNReader instance is ready to perform network requests to the provided CDN base URL.
func NewCDNReader(logger *slog.Logger, baseURL string, restyClientBase *resty.Client) *CDNReader {
	restyClient := httpx.NewRestyClientFactoryWithBase(restyClientBase).New()
	headers := httpx.NewHeaders().UserAgent().Value("go-wallet-toolbox")

	restyClient = restyClient.
		SetHeaders(headers).
		SetLogger(logging.RestyAdapter(logger)).
		SetDebug(logging.IsDebug(logger)).
		SetBaseURL(baseURL).
		SetRetryCount(3)

	return &CDNReader{
		logger: logging.Child(logger, "chaintracks_cdn_reader"),
		resty:  restyClient,
	}
}

// FetchBulkHeaderFilesInfo retrieves metadata about available bulk block header files for the specified BSV network.
// It returns information such as the root folder, JSON file name, headers per file, and a list of file details.
// An error is returned if fetching or decoding the response fails, or if the response is empty.
func (c *CDNReader) FetchBulkHeaderFilesInfo(ctx context.Context, chain defs.BSVNetwork) (*BulkHeaderFilesInfo, error) {
	var result *BulkHeaderFilesInfo
	resp, err := c.resty.R().
		SetContext(ctx).
		SetHeaders(httpx.NewHeaders().AcceptJSON().ContentTypeJSON()).
		SetResult(&result).
		SetPathParam("chain", string(chain)).
		Get("/{chain}NetBlockHeaders.json")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch bulk header files info: %w", err)
	}

	if !resp.IsSuccess() {
		return nil, fmt.Errorf("failed to fetch bulk header files info: received status code %d", resp.StatusCode())
	}

	if result == nil {
		return nil, fmt.Errorf("failed to fetch bulk header files info: empty response")
	}

	return result, nil
}

func (c *CDNReader) DownloadBulkHeaderFile(ctx context.Context, filename string) ([]byte, error) {
	resp, err := c.resty.R().
		SetContext(ctx).
		SetDebug(false). //NOTE: Disable debug for large binary downloads
		SetHeaders(httpx.NewHeaders().Accept().Value("application/octet-stream")).
		Get(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to download bulk header file %s: %w", filename, err)
	}

	if !resp.IsSuccess() {
		return nil, fmt.Errorf("failed to download bulk header file %s: received status code %d", filename, resp.StatusCode())
	}

	return resp.Body(), nil
}
