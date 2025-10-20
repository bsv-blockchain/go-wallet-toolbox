package bitails

import (
	"context"
	"fmt"
	"net/http"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/bitails/internal/dto"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func (b *Bitails) ChainHeaderByHeight(ctx context.Context, height uint32) (*wdk.ChainBlockHeader, error) {
	url, err := blockByHeight(b.url, height)
	if err != nil {
		return nil, fmt.Errorf("failed to build URL to retrieve block by height from Bitails: %w", err)
	}

	var dst dto.BlockHeaderByHeight
	req := b.httpClient.
		R().
		SetContext(ctx).
		SetResult(&dst)

	res, err := req.Get(url)
	if err != nil {
		return nil, fmt.Errorf("unexpected response from API (URL: %s): %w", url, err)
	}
	if res.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("unexpected response from API (URL: %s, code: %d)", url, res.StatusCode())
	}

	if dst.IsZero() {
		return nil, fmt.Errorf("expected a non-empty block header at height %d", height)
	}

	base, err := dst.ConvertToChainBlockHeader()
	if err != nil {
		return nil, fmt.Errorf("failed to convert block header response by height from Bitails to a chain base block header: %w", err)
	}
	return base, nil
}
