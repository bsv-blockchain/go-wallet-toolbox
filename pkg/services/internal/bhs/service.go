package bhs

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

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
		AddRetryCondition(func(res *resty.Response, err error) bool {
			return res.StatusCode() == http.StatusTooManyRequests
		}).
		Get(url)

	if err != nil {
		return nil, fmt.Errorf("error while fetching block header from Block Headers Servcie API (URL: %s): %w", url, err)
	}

	if res.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("unexpected response from  Block Headers Servcie API (URL: %s): status code %d", url, res.StatusCode())
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
		panic(err)
	}

	err = config.Validate()
	if err != nil {
		panic(err)
	}

	return &BlockHeadersService{
		httpClient: newRestyHTTPClient(httpClient, logger, network, config),
		cfg:        &config,
	}
}

func newRestyHTTPClient(httpClient *resty.Client, logger *slog.Logger, network defs.BSVNetwork, config defs.BHS) *resty.Client {
	const (
		retries         = 2
		retriesWaitTime = 2 * time.Second // TODO: Move this section to shared pkg.
	)

	child := logging.
		Child(logger, ServiceName).
		With(slog.String("network", string(network)))

	headers := httpx.NewHeaders().
		AcceptJSON().
		UserAgent().Value("go-wallet-toolbox").
		Authorization().IfNotEmpty(config.APIKey)

	return httpClient.Clone().
		SetRetryCount(retries).
		SetBaseURL(config.URL).
		SetRetryWaitTime(retriesWaitTime).
		SetRetryMaxWaitTime(retries * retriesWaitTime).
		SetHeaders(headers).
		SetLogger(logging.RestyAdapter(child)).
		SetDebug(child.Enabled(context.Background(), slog.LevelDebug))
}
