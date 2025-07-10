package bhs

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/bhs/internal/dto"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/httpx"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-resty/resty/v2"
	"github.com/go-softwarelab/common/pkg/to"
)

const ServiceName = "BlockHeadersService"

type BlockHeadersService struct {
	httpClient *resty.Client
	cfg        *defs.BHS
}

func NewBlockHeadersService(httpClient *resty.Client, logger *slog.Logger, network defs.BSVNetwork, config defs.BHS) *BlockHeadersService {
	err := network.Validate()
	if err != nil {
		panic(fmt.Sprintf("invalid BSV network configuration: %s", err.Error()))
	}

	err = config.Validate()
	if err != nil {
		panic(fmt.Sprintf("invalid BHS configuration: %s", err.Error()))
	}

	child := logging.
		Child(logger, ServiceName).
		With(slog.String("network", string(network)))

	headers := httpx.NewHeaders().
		AcceptJSON().
		UserAgent().Value("go-wallet-toolbox").
		Authorization().IfNotEmpty(bearerHeader(config.APIKey))

	client := httpClient.SetBaseURL(config.URL).
		SetHeaders(headers).
		SetLogger(logging.RestyAdapter(child)).
		SetDebug(logging.IsDebug(logger))

	return &BlockHeadersService{
		httpClient: client,
		cfg:        &config,
	}
}

func (b *BlockHeadersService) FindChainTipHeader(ctx context.Context) (*wdk.ChainBlockHeader, error) {
	var block dto.TipStateResponse
	url, err := tipLongestURL(b.cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("error while building tip URL for Block Headers Service API: %w", err)
	}
	res, err := b.
		httpClient.
		R().
		SetContext(ctx).
		SetResult(&block).
		Get(url)

	if err != nil {
		return nil, fmt.Errorf("error while fetching block header from Block Headers Service API (URL: %s): %w", url, err)
	}

	if res.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("unexpected response from  Block Headers Service API (URL: %s): status code %d", url, res.StatusCode())
	}

	if block.IsZero() {
		return nil, fmt.Errorf("unexpected response from  Block Headers Service API (URL: %s). Received an empty block header response", url)
	}

	return block.ConvertToChainBlockHeader(), nil
}

// GetHeight returns the best-chain height reported by the Block-Headers
// Service (`/chain/tip/longest`).
func (b *BlockHeadersService) GetHeight(ctx context.Context) (uint32, error) {
	var tip dto.TipStateResponse
	url, err := tipLongestURL(b.cfg.URL)
	if err != nil {
		return 0, fmt.Errorf("bhs: failed to build tip URL: %w", err)
	}

	res, err := b.
		httpClient.
		R().
		SetContext(ctx).
		SetResult(&tip).
		Get(url)
	if err != nil {
		return 0, fmt.Errorf("bhs: height query failed (%s): %w", url, err)
	}
	if res.StatusCode() != http.StatusOK {
		return 0, fmt.Errorf("bhs: unexpected HTTP %d for %s", res.StatusCode(), url)
	}
	if tip.IsZero() || tip.Height == 0 {
		return 0, fmt.Errorf("bhs: empty /chain/tip/longest response")
	}

	height, err := to.UInt32(tip.Height)
	if err != nil {
		return 0, fmt.Errorf("bhs: invalid height %d in /chain/tip/longest response: %w", tip.Height, err)
	}

	return height, nil
}
