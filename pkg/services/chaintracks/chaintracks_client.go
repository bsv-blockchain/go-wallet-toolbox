package chaintracks

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/httpx"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-resty/resty/v2"
	"github.com/go-softwarelab/common/pkg/to"
)

// Client provides methods for interacting with a ChainTracks BSV network backend.
// It is configured with a logger, network type, service URL, and HTTP REST client.
// Client allows querying network info and interacting with blockchain data over HTTP APIs.
// Constructed using NewClient, it enforces valid BSV network values and a non-empty URL.
// Internal logger is scoped for diagnostics; HTTP client is configured for JSON APIs.
type Client struct {
	logger *slog.Logger
	chain  defs.BSVNetwork
	url    string
	resty  *resty.Client
}

// NewClient creates and returns a new Client configured for the specified BSV network and ChainTracks service URL.
// It validates the BSV network value and panics if the network is invalid or the URL is empty.
// The logger argument is used for debug logging, optionally specialized for ChainTracks diagnostics.
// Additional client options can be supplied to customize HTTP REST configuration.
// Returns a pointer to a ready-to-use Client for network operations.
func NewClient(logger *slog.Logger, chain defs.BSVNetwork, url string, opts ...func(options *ClientOptions)) *Client {
	options := to.OptionsWithDefault(ClientOptions{
		RestyClientFactory: httpx.NewRestyClientFactory(),
	}, opts...)

	if err := chain.Validate(); err != nil {
		panic(fmt.Errorf("invalid chain %q: %w", chain, err))
	}

	url = strings.TrimRight(url, "/")
	if url == "" {
		panic("chaintracks url is required")
	}

	logger = logging.Child(logger, "chaintracks_client")

	restyClient := options.RestyClientFactory.New()
	headers := httpx.NewHeaders().
		AcceptJSON().
		ContentTypeJSON().
		UserAgent().Value("go-wallet-toolbox")
	restyClient = restyClient.
		SetHeaders(headers).
		SetLogger(logging.RestyAdapter(logger)).
		SetDebug(logging.IsDebug(logger))

	return &Client{
		logger: logger,
		chain:  chain,
		url:    url,
		resty:  restyClient,
	}
}

// Chain returns the BSV network this client is configured for.
// It just returns the value passed to NewClient, it does not make any network requests.
// To check if the server actually supports this network, call GetInfo().
func (c *Client) Chain() defs.BSVNetwork {
	return c.chain
}

// GetInfo queries the backend service for network details and configuration information.
// GetInfo returns an InfoResponse with chain id, block heights, ingest endpoints, and installed package metadata.
// GetInfo returns an error if the request fails, responds with non-200 status, or response indicates failure.
func (c *Client) GetInfo(ctx context.Context) (*InfoResponse, error) {
	if resp, err := getRequest[InfoResponse](c, ctx, c.url+"/getInfo"); err != nil {
		return nil, fmt.Errorf("getinfo: %w", err)
	} else {
		return resp, nil
	}
}

// GetPresentHeight queries the backend for the current blockchain height and returns it as a uint value.
// Returns an error if the request fails, the HTTP status is not 200, or the response indicates failure.
func (c *Client) GetPresentHeight(ctx context.Context) (uint32, error) {
	if resp, err := getRequest[uint32](c, ctx, c.url+"/getPresentHeight"); err != nil {
		return 0, fmt.Errorf("getPresentHeight: %w", err)
	} else {
		return *resp, nil
	}
}

// FindChainTipHashHex retrieves the current chain tip block hash in hex format from the backend service.
// Returns the hash as a lowercase hexadecimal string, or an error if the request or response fails.
func (c *Client) FindChainTipHashHex(ctx context.Context) (string, error) {
	if resp, err := getRequest[string](c, ctx, c.url+"/findChainTipHashHex"); err != nil {
		return "", fmt.Errorf("findChainTipHashHex: %w", err)
	} else {
		return *resp, nil
	}
}

// FindChainTipHeaderHex retrieves the current chain tip block header in hex format from the backend service.
// Returns a BlockHeader struct with header fields, or an error if the request or response is unsuccessful.
func (c *Client) FindChainTipHeaderHex(ctx context.Context) (*BlockHeader, error) {
	if resp, err := getRequest[BlockHeader](c, ctx, c.url+"/findChainTipHeaderHex"); err != nil {
		return nil, fmt.Errorf("findChainTipHeaderHex: %w", err)
	} else {
		return resp, nil
	}
}

// FindHeaderHexForHeight queries the backend for the block header at the given height and returns a BlockHeader struct.
// Returns an error if the request is unsuccessful, the height is not found, or the backend returns a non-200 status.
// The returned BlockHeader includes header fields, chain height, and block hash, or nil if not found.
func (c *Client) FindHeaderHexForHeight(ctx context.Context, height uint32) (*BlockHeader, error) {
	if resp, err := getRequest[BlockHeader](c, ctx, fmt.Sprintf("%s/findHeaderHexForHeight?height=%d", c.url, height)); err != nil {
		return nil, fmt.Errorf("findHeaderHexForHeight: %w", err)
	} else {
		return resp, nil
	}
}

func getRequest[T any](c *Client, ctx context.Context, url string) (*T, error) {
	var resp ResponseFrame[T]
	result, err := c.resty.R().
		SetContext(ctx).
		SetResult(&resp).
		Get(url)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	} else if result.StatusCode() != 200 {
		c.logger.DebugContext(ctx, "chaintracks non-200 HTTP status", "status", result.StatusCode(), "body", result.String())
		return nil, fmt.Errorf("HTTP status: %d", result.StatusCode())
	} else if !resp.IsSuccess() {
		return nil, fmt.Errorf("response not successful: status: %q", resp.Status)
	} else if resp.IsNotFound() {
		return nil, fmt.Errorf("response value missing: %w", wdk.ErrNotFoundError)
	}

	return resp.Value, nil
}
