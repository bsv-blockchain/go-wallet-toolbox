package ingest

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/httpx"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/whatsonchain"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-resty/resty/v2"
)

type wocClient struct {
	logger *slog.Logger
	resty  *resty.Client
}

func newWocClient(logger *slog.Logger, chain defs.BSVNetwork, apiKey string, restyClient *resty.Client) *wocClient {
	url, err := whatsonchain.MakeBaseURL(chain)
	if err != nil {
		panic(fmt.Sprintf("failed to build base URL for WhatsOnChain: %s", err.Error()))
	}

	headers := httpx.NewHeaders().
		AcceptJSON().
		ContentTypeJSON().
		UserAgent().Value("go-wallet-toolbox").
		Authorization().IfNotEmpty(apiKey)

	restyClient = restyClient.
		SetHeaders(headers).
		SetLogger(logging.RestyAdapter(logger)).
		SetDebug(logging.IsDebug(logger)).
		SetBaseURL(url)

	return &wocClient{
		logger: logger,
		resty:  restyClient,
	}
}

func (c *wocClient) GetHeaderByHash(ctx context.Context, hash string) (*WOCBlockHeaderDTO, error) {
	path := fmt.Sprintf("/block/%s/header", hash)

	var hdrResp WOCBlockHeaderDTO
	res, err := c.resty.R().
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

	return &hdrResp, nil
}

func (c *wocClient) GetPresentHeight(ctx context.Context) (uint, error) {
	path := "/chain/info"

	var infoResp blockOnlyChainInfoDTO
	res, err := c.resty.R().
		SetContext(ctx).
		SetResult(&infoResp).
		Get(path)

	if err != nil {
		return 0, fmt.Errorf("failed to fetch chain info: %w", err)
	}
	if res.StatusCode() != http.StatusOK {
		return 0, fmt.Errorf("unexpected status code %d fetching chain info", res.StatusCode())
	}

	return infoResp.Blocks, nil
}

func (c *wocClient) GetLastHeaders(ctx context.Context) (WOCBlockHeadersDTO, error) {
	path := "/block/headers"

	var headersResponse WOCBlockHeadersDTO
	res, err := c.resty.R().
		SetContext(ctx).
		SetResult(&headersResponse).
		Get(path)

	if err != nil {
		return nil, fmt.Errorf("failed to fetch block headers: %w", err)
	}
	if res.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d fetching block headers", res.StatusCode())
	}

	return headersResponse, nil
}

func (c *wocClient) GetHeadersResourceList(ctx context.Context) (*WOCHeadersResourcesDTO, error) {
	path := "/block/headers/resources"

	var response *WOCHeadersResourcesDTO
	resp, err := c.resty.R().
		SetContext(ctx).
		SetResult(&response).
		Get(path)

	if err != nil {
		return nil, fmt.Errorf("failed to fetch bulk header files info: %w", err)
	}

	if !resp.IsSuccess() {
		return nil, fmt.Errorf("failed to fetch bulk header files info: received status code %d", resp.StatusCode())
	}

	if response == nil || len(response.Files) == 0 {
		return nil, fmt.Errorf("failed to fetch bulk header files info: empty response")
	}

	return response, nil
}

func (c *wocClient) GetContentDispositionFilename(ctx context.Context, fileURL string) (string, error) {
	resp, err := c.resty.R().
		SetContext(ctx).
		Head(fileURL)
	if err != nil {
		return "", fmt.Errorf("failed to fetch latest bulk header file URL: %w", err)
	}

	if !resp.IsSuccess() {
		return "", fmt.Errorf("failed to fetch latest bulk header file URL: received status code %d", resp.StatusCode())
	}

	return resp.Header().Get("Content-Disposition"), nil
}

func (c *wocClient) DownloadHeaderFile(ctx context.Context, fileURL string) ([]byte, error) {
	resp, err := c.resty.R().
		SetContext(ctx).
		SetDebug(false). //NOTE: Disable debug for large binary downloads
		SetHeaders(httpx.NewHeaders().Accept().Value("application/octet-stream")).
		Get(fileURL)
	if err != nil {
		return nil, fmt.Errorf("failed to download bulk header file: %w", err)
	}

	if !resp.IsSuccess() {
		return nil, fmt.Errorf("failed to download bulk header file: received status code %d", resp.StatusCode())
	}

	return resp.Body(), nil
}
