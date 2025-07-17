package bhs

import (
	"context"
	"fmt"
	"net/http"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/httpx"
	"github.com/go-resty/resty/v2"
)

func (b *BlockHeadersService) doGET(ctx context.Context, url string, result any) (*resty.Response, error) {
	res, err := b.httpClient.R().
		SetContext(ctx).
		SetResult(result).
		Get(url)
	if err != nil {
		return nil, fmt.Errorf("%s: GET %s: %w", ServiceName, url, err)
	}
	if res.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("%s: unexpected HTTP %d for GET %s", ServiceName, res.StatusCode(), url)
	}
	return res, nil
}

func (b *BlockHeadersService) doPOST(ctx context.Context, url string, body, result any) (*resty.Response, error) {
	res, err := b.httpClient.R().
		SetContext(ctx).
		SetBody(body).
		SetResult(result).
		AddRetryCondition(httpx.RetryOnErrOr5xx).
		Post(url)
	if err != nil {
		return nil, fmt.Errorf("%s: POST %s failed: %w", ServiceName, url, err)
	}
	if res.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("%s: unexpected HTTP %d for POST %s", ServiceName, res.StatusCode(), url)
	}
	return res, nil
}
