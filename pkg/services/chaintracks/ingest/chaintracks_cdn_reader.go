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

// BulkHeaderFilesInfo represents metadata about a collection of bulk block header files and their containing folder.
type BulkHeaderFilesInfo struct {
	RootFolder     string               `json:"rootFolder"`
	JSONFilename   string               `json:"jsonFilename"`
	HeadersPerFile int                  `json:"headersPerFile"`
	Files          []BulkHeaderFileInfo `json:"files"`
}

// BulkHeaderFileInfo contains metadata related to a single bulk block header file for a specific blockchain network.
type BulkHeaderFileInfo struct {
	FileName      string           `json:"fileName"`
	FirstHeight   uint             `json:"firstHeight"`
	Count         int              `json:"count"`
	PrevChainWork string           `json:"prevChainWork"`
	LastChainWork string           `json:"lastChainWork"`
	PrevHash      string           `json:"prevHash"`
	LastHash      *string          `json:"lastHash,omitempty"`
	FileHash      *string          `json:"fileHash,omitempty"`
	Chain         *defs.BSVNetwork `json:"chain,omitempty"`
	SourceURL     *string          `json:"sourceUrl,omitempty"`
}

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
		SetBaseURL(baseURL)

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
