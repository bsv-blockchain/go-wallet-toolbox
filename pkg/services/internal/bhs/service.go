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

type BlockHeadersService struct {
	httpClient *resty.Client
	cfg        *defs.BHS
}

const ServiceName = defs.BHSServiceName

func New(httpClient *resty.Client, logger *slog.Logger, network defs.BSVNetwork, config defs.BHS) *BlockHeadersService {
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

func (b *BlockHeadersService) ChainHeaderByHeight(ctx context.Context, height uint32) (*wdk.ChainBlockHeader, error) {
	url, err := headerByHeight(b.cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("error building URL: %w", err)
	}

	var blocks []dto.BlockHeaderByHeightResponse
	req := b.httpClient.R().
		SetContext(ctx).
		SetResult(&blocks).
		SetQueryParam("height", fmt.Sprint(height)).
		SetQueryParam("count", "1")

	res, err := req.Get(url)
	if err != nil {
		return nil, fmt.Errorf("unexpected response from API (URL: %s): %w", req.URL, err)
	}
	if res.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("unexpected response from API (URL: %s, code: %d)", req.URL, res.StatusCode())
	}

	if len(blocks) != 1 {
		return nil, fmt.Errorf("expected a single block header at height %d, but received %d headers instead. Verify the BHS API and query parameters used", height, len(blocks))
	}

	first := blocks[0]
	if first.IsZero() {
		return nil, fmt.Errorf("expected a non-empty block header at height %d. Verify the BHS API and query parameters used", height)
	}

	return first.ConvertChainBlockHeader(), nil
}

func (b *BlockHeadersService) IsValidRootForHeight(ctx context.Context, root *chainhash.Hash, height uint32) (bool, error) {
	url, err := verifyMerkleRootURL(b.cfg.URL)
	if err != nil {
		return false, fmt.Errorf("error building URL: %w", err)
	}

	req := []dto.MerkleRootVerifyItem{{
		BlockHeight: height,
		MerkleRoot:  root.String(),
	}}

	var resp dto.MerkleRootVerifyResp
	res, err := b.httpClient.R().
		SetContext(ctx).
		SetBody(req).
		SetResult(&resp).
		AddRetryCondition(httpx.RetryOnErrOr5xx).
		Post(url)
	if err != nil {
		return false, fmt.Errorf("%s: POST %s failed: %w", ServiceName, url, err)
	}
	if res.StatusCode() != http.StatusOK {
		return false, fmt.Errorf("%s: unexpected HTTP %d for POST %s", ServiceName, res.StatusCode(), url)
	}

	switch {
	case resp.ConfirmationState.IsConfirmed():
		return true, nil
	case resp.ConfirmationState.IsInvalid():
		return false, nil
	case resp.ConfirmationState.IsUnableToVerify():
		return false, fmt.Errorf("unable to verify merkle root (state=%q)", resp.ConfirmationState)
	default:
		return false, fmt.Errorf("unexpected confirmation state %q", resp.ConfirmationState)
	}
}

// CurrentHeight returns the best-chain height reported by the Block-Headers
// Service (`/chain/tip/longest`).
func (b *BlockHeadersService) CurrentHeight(ctx context.Context) (uint32, error) {
	tip, err := b.FindChainTipHeader(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to find chain tip header: %w", err)
	}

	height, err := to.UInt32(tip.Height)
	if err != nil {
		return 0, fmt.Errorf("failed to convert height %d to uint32: %w", tip.Height, err)
	}
	return height, nil
}

func (b *BlockHeadersService) FindChainTipHeader(ctx context.Context) (*wdk.ChainBlockHeader, error) {
	var block dto.TipStateResponse
	url, err := tipLongestURL(b.cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("error building URL: %w", err)
	}

	res, err := b.httpClient.R().
		SetContext(ctx).
		SetResult(&block).
		Get(url)
	if err != nil {
		return nil, fmt.Errorf("%s: GET %s: %w", ServiceName, url, err)
	}
	if res.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("%s: unexpected HTTP %d for GET %s", ServiceName, res.StatusCode(), url)
	}

	if block.IsZero() {
		return nil, fmt.Errorf("unexpected response from API (URL: %s). Received an empty tip state response", url)
	}

	return block.ConvertToChainBlockHeader(), nil
}
