package bhs

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/logging"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/services/internal/bhs/internal/dto"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/services/internal/httpx"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/go-resty/resty/v2"
)

const ServiceName = "BlockHeadersService"

type BlockHeadersService struct {
	httpClient *resty.Client
	cfg        *defs.BHS
}

func (b *BlockHeadersService) FindChainTipHeader(ctx context.Context) (*wdk.ChainBlockHeader, error) {
	var block dto.BlockHeader
	url := fmt.Sprintf("%s/chain/tip/longest", b.cfg.URL)
	res, err := b.
		httpClient.
		R().
		SetContext(ctx).
		SetResult(&block).
		AddRetryCondition(httpx.RetryOnTooManyRequestsStatus).
		Get(url)

	if err != nil {
		return nil, fmt.Errorf("error while fetching block header from Block Headers Service API (URL: %s): %w", url, err)
	}

	if res.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("unexpected response from  Block Headers Service API (URL: %s): status code %d", url, res.StatusCode())
	}

	if block.IsZero() {
		return nil, fmt.Errorf("unexpected response from  Block Headers Service API (URL: %s). Received an empty block header response.", url)
	}

	return block.ConvertToChainBlockHeader(), nil
}

func NewBlockHeadersService(httpClient *resty.Client, logger *slog.Logger, network defs.BSVNetwork, config defs.BHS) *BlockHeadersService {
	if httpClient == nil {
		panic("httpClient is required")
	}
	if logger == nil {
		panic("logger is required")
	}

	err := network.Validate()
	if err != nil {
		panic(fmt.Sprintf("invalid BSV network configuration: %s", err.Error()))
	}

	err = config.Validate()
	if err != nil {
		panic(fmt.Sprintf("invalid BHS configuration: %s", err.Error()))
	}

	return &BlockHeadersService{
		httpClient: newRestyHTTPClient(httpClient, logger, network, config),
		cfg:        &config,
	}
}

func newRestyHTTPClient(httpClient *resty.Client, logger *slog.Logger, network defs.BSVNetwork, config defs.BHS) *resty.Client {

	child := logging.
		Child(logger, ServiceName).
		With(slog.String("network", string(network)))

	headers := httpx.NewHeaders().
		AcceptJSON().
		UserAgent().Value("go-wallet-toolbox").
		Authorization().IfNotEmpty(config.APIKey)

	return httpClient.Clone().
		SetRetryCount(httpx.DefaultRetryCount).
		SetBaseURL(config.URL).
		SetRetryWaitTime(httpx.DefaultRetryInterval).
		SetRetryMaxWaitTime(httpx.DefaultRetryCount * httpx.DefaultRetryInterval).
		SetHeaders(headers).
		SetLogger(logging.RestyAdapter(child)).
		SetDebug(child.Enabled(context.Background(), slog.LevelDebug))
}
