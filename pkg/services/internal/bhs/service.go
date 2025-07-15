package bhs

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/bsv-blockchain/go-sdk/chainhash"
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

func (b *BlockHeadersService) FindChainTipHeader(ctx context.Context) (*wdk.ChainBlockHeader, error) {
	var block dto.TipStateResponse
	url, err := tipLongestURL(b.cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("failed for service %s to build tip URL: %w", ServiceName, err)
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
		return nil, fmt.Errorf("unexpected response from Block Headers Service API (URL: %s): status code %d", url, res.StatusCode())
	}

	if block.IsZero() {
		return nil, fmt.Errorf("unexpected response from Block Headers Service API (URL: %s). Received an empty block header response", url)
	}

	return block.ConvertToChainBlockHeader(), nil
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

func (b *BlockHeadersService) IsValidRootForHeight(ctx context.Context, root *chainhash.Hash, height uint32) (bool, error) {
	url, err := verifyMerkleRootURL(b.cfg.URL)
	if err != nil {
		return false, fmt.Errorf("failed for service %s: build verify URL: %w", ServiceName, err)
	}

	req := []dto.MerkleRootVerifyItem{{
		BlockHeight: height,
		MerkleRoot:  root.String(),
	}}

	var resp []dto.MerkleRootVerifyResp
	res, err := b.
		httpClient.
		R().
		SetContext(ctx).
		SetBody(req).
		SetResult(&resp).
		AddRetryCondition(httpx.RetryOnErrOr5xx).
		Post(url)
	if err != nil {
		return false, fmt.Errorf("failed for service %s: verify request failed (%s): %w", ServiceName, url, err)
	}
	if res.StatusCode() != http.StatusOK {
		return false, fmt.Errorf("failed for service %s: unexpected HTTP %d for %s", ServiceName, res.StatusCode(), url)
	}
	if len(resp) != 1 {
		return false, fmt.Errorf("failed for service %s: verify response has %d elements, want 1", ServiceName, len(resp))
	}

	switch {
	case resp[0].ConfirmationState.IsConfirmed():
		return true, nil
	case resp[0].ConfirmationState.IsInvalid():
		return false, nil
	default:
		return false, fmt.Errorf("failed for service %s: unable to verify merkle root (state=%q)", ServiceName, resp[0].ConfirmationState)
	}
}

// CurrentHeight returns the best-chain height reported by the Block-Headers
// Service (`/chain/tip/longest`).
func (b *BlockHeadersService) CurrentHeight(ctx context.Context) (uint32, error) {
	var tip dto.TipStateResponse
	url, err := tipLongestURL(b.cfg.URL)
	if err != nil {
		return 0, fmt.Errorf("failed for service %s to build tip URL: %w", ServiceName, err)
	}

	res, err := b.
		httpClient.
		R().
		SetContext(ctx).
		SetResult(&tip).
		Get(url)
	if err != nil {
		return 0, fmt.Errorf("failed for service %s: height query failed (%s): %w", ServiceName, url, err)
	}
	if res.StatusCode() != http.StatusOK {
		return 0, fmt.Errorf("failed for service %s: unexpected HTTP %d for %s", ServiceName, res.StatusCode(), url)
	}
	if tip.IsZero() || tip.Height == 0 {
		return 0, fmt.Errorf("failed for service %s: empty /chain/tip/longest response", ServiceName)
	}

	height, err := to.UInt32(tip.Height)
	if err != nil {
		return 0, fmt.Errorf("failed for service %s: invalid height %d in /chain/tip/longest response: %w", ServiceName, tip.Height, err)
	}

	return height, nil
}
