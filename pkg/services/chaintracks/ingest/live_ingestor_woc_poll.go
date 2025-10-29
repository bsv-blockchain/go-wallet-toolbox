package ingest

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/httpx"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/whatsonchain"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-resty/resty/v2"
	"github.com/go-softwarelab/common/pkg/to"
)

const (
	genesisAsPrevBlockHash = "0000000000000000000000000000000000000000000000000000000000000000"
)

// LiveIngestorWocPoll provides functionality for polling block header data from an external source, such as WhatsOnChain.
type LiveIngestorWocPoll struct {
	logger *slog.Logger
	config defs.WOCPollIngestorConfig
	resty  *resty.Client
}

// NewLiveIngestorWocPoll creates a new LiveIngestorWocPoll using the provided logger, config, and optional client options.
// It initializes a Resty HTTP client configured with default headers, user agent, and API key authorization if set.
// Panics if the WhatsOnChain base URL cannot be built for the specified chain network in the config.
// Returns a pointer to the initialized LiveIngestorWocPoll struct, ready for external data polling operations.
func NewLiveIngestorWocPoll(logger *slog.Logger, config defs.WOCPollIngestorConfig, opts ...func(options *ClientOptions)) *LiveIngestorWocPoll {
	logger = logging.Child(logger, "live_ingestor_woc_poll")

	options := to.OptionsWithDefault(ClientOptions{
		RestyClientFactory: httpx.NewRestyClientFactory(),
	}, opts...)

	url, err := whatsonchain.MakeBaseURL(config.Chain)
	if err != nil {
		panic(fmt.Sprintf("failed to build base URL for WhatsOnChain: %s", err.Error()))
	}

	restyClient := options.RestyClientFactory.New()
	headers := httpx.NewHeaders().
		AcceptJSON().
		ContentTypeJSON().
		UserAgent().Value("go-wallet-toolbox").
		Authorization().IfNotEmpty(config.APIKey)

	restyClient = restyClient.
		SetHeaders(headers).
		SetLogger(logging.RestyAdapter(logger)).
		SetDebug(logging.IsDebug(logger)).
		SetBaseURL(url)

	return &LiveIngestorWocPoll{
		logger: logger,
		config: config,
		resty:  restyClient,
	}
}

// GetHeaderByHash retrieves a Bitcoin block header from an external data source using its hash.
// Returns a ChainBlockHeader and error if the header could not be fetched or parsed.
// If the block is not found, returns wdk.ErrNotFoundError as wrapped error.
// The hash parameter must be a valid block hash as a hex string.
// PreviousHash is set to a predefined value if the block is the genesis block.
func (ing *LiveIngestorWocPoll) GetHeaderByHash(ctx context.Context, hash string) (*wdk.ChainBlockHeader, error) {
	path := fmt.Sprintf("/block/%s/header", hash)

	var hdrResp WOCBlockHeaderDTO
	res, err := ing.resty.R().
		SetContext(ctx).
		SetResult(&hdrResp).
		Get(path)

	if err != nil {
		return nil, fmt.Errorf("failed to fetch block header: %w", err)
	}
	if res.StatusCode() != http.StatusOK {
		if res.StatusCode() == http.StatusNotFound {
			return nil, fmt.Errorf("block header not found for hash %s: %w", hash, wdk.ErrNotFoundError)
		}
		return nil, fmt.Errorf("unexpected status code %d fetching block header", res.StatusCode())
	}

	if hdrResp.PrevBlock == "" {
		hdrResp.PrevBlock = genesisAsPrevBlockHash
	}

	wdkBlockHeader, err := hdrResp.ToWDK()
	if err != nil {
		return nil, fmt.Errorf("failed to convert block header DTO to WDK format: %w", err)
	}

	return wdkBlockHeader, nil
}

func (ing *LiveIngestorWocPoll) GetLast10Headers(ctx context.Context) ([]*wdk.ChainBlockHeader, error) {
	path := "/block/headers"

	var headersResponse WOCBlockHeadersDTO
	res, err := ing.resty.R().
		SetContext(ctx).
		SetResult(&headersResponse).
		Get(path)

	if err != nil {
		return nil, fmt.Errorf("failed to fetch block headers: %w", err)
	}
	if res.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d fetching block headers", res.StatusCode())
	}

	wdkHeaders, err := headersResponse.ToWDK()
	if err != nil {
		return nil, fmt.Errorf("failed to convert block headers DTO to WDK format: %w", err)
	}

	return wdkHeaders, nil
}

func (ing *LiveIngestorWocPoll) bitsStrToUint32(bitsStr string) (uint32, error) {
	bitsNum, err := strconv.ParseUint(bitsStr, 16, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid bits value %s: %w", bitsStr, err)
	}

	return uint32(bitsNum), nil
}
