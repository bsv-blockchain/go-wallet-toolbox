package chaintracks

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/httpx"
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

	if url == "" {
		panic("chaintracks url is required")
	}
	url = strings.TrimRight(url, "/")

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
// To check if the server actually supports this network, use the GetInfo.
func (c *Client) Chain() defs.BSVNetwork {
	return c.chain
}

// GetInfo queries the backend service for network details and configuration information.
// GetInfo returns an InfoResponse with chain id, block heights, ingest endpoints, and installed package metadata.
// GetInfo returns an error if the request fails, responds with non-200 status, or response indicates failure.
func (c *Client) GetInfo(ctx context.Context) (*InfoResponse, error) {
	var resp ResponseFrame[InfoResponse]
	if result, err := c.resty.R().
		SetContext(ctx).
		SetResult(&resp).
		Get(c.url + "/getInfo"); err != nil {
		return nil, fmt.Errorf("getinfo request: %w", err)
	} else if result.StatusCode() != 200 {
		c.logger.DebugContext(ctx, "chaintracks getinfo non-200 status", "status", result.StatusCode(), "body", result.String())
		return nil, fmt.Errorf("getinfo status: %d", result.StatusCode())
	} else if !resp.IsSuccess() {
		return nil, fmt.Errorf("getinfo response not successful: status: %q", resp.Status)
	}

	return resp.Value, nil
}
